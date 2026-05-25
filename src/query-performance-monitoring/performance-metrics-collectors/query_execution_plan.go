package performancemetricscollectors

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/bitly/go-simplejson"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	dbutils "github.com/newrelic/nri-mysql/src/dbutils"
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// PopulateExecutionPlans populates execution plans for the given queries.
func PopulateExecutionPlans(db utils.DataSource, queryGroups map[string][]utils.IndividualQueryMetrics, i *integration.Integration, args arguments.ArgumentList, flavor utils.DatabaseFlavor) {
	var events []utils.QueryPlanMetrics

	for dbName, queries := range queryGroups {
		dsn := dbutils.GenerateDSN(args, dbName)
		// Open the DB connection
		db, err := utils.OpenSQLXDB(dsn)
		if err != nil {
			log.Error("Error opening database connection: %v", err)
			continue
		}
		defer db.Close()

		for _, query := range queries {
			tableIngestionDataList, err := processExecutionPlanMetrics(db, query, flavor)
			if err != nil {
				log.Error("Error processing execution plan metrics: %v", err)
				continue
			}
			events = append(events, tableIngestionDataList...)
		}
	}

	// Return if no metrics are collected
	if len(events) == 0 {
		return
	}

	// Set the execution plan metrics in the integration entity and ingest them
	err := SetExecutionPlanMetrics(i, args, events)
	if err != nil {
		log.Error("Error publishing execution plan metrics: %v", err)
		return
	}
}

// processExecutionPlanMetrics processes the execution plan metrics for a given query.
func processExecutionPlanMetrics(db utils.DataSource, query utils.IndividualQueryMetrics, flavor utils.DatabaseFlavor) ([]utils.QueryPlanMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QueryPlanTimeoutDuration)
	defer cancel()

	queryID, err := getQueryID(query)
	if err != nil {
		log.Warn("Query ID is nil, skipping. Error: %v", err)
		return []utils.QueryPlanMetrics{}, err
	}
	queryText, err := getQueryText(query, queryID)
	if err != nil {
		return []utils.QueryPlanMetrics{}, err
	}

	if !isSupportedStatement(queryText) {
		log.Warn("Skipping unsupported query for EXPLAIN: %s. Query ID: %s", queryText, queryID)
		return []utils.QueryPlanMetrics{}, nil
	}

	if strings.Contains(queryText, "?") {
		log.Warn("Skipping query with placeholders for EXPLAIN: %s. Query ID: %s", queryText, queryID)
		return []utils.QueryPlanMetrics{}, nil
	}

	execPlanJSON, err := executeExplainQuery(ctx, db, queryText)
	if err != nil {
		return []utils.QueryPlanMetrics{}, err
	}

	// MariaDB's EXPLAIN FORMAT=JSON can produce duplicate keys (e.g., multiple "table" siblings)
	// which Go's encoding/json silently drops. Pre-process only for MariaDB to rename duplicates.
	processedJSON := execPlanJSON
	if flavor == utils.DatabaseFlavorMariaDB {
		processedJSON, err = deduplicateJSONKeys(execPlanJSON)
		if err != nil {
			log.Error("Error deduplicating JSON keys for query '%s': %v", queryText, err)
			return []utils.QueryPlanMetrics{}, err
		}
	}

	escapedJSON, err := escapeAllStringsInJSON(processedJSON)
	if err != nil {
		log.Error("Error escaping strings in JSON for query '%s': %v", queryText, err)
		return []utils.QueryPlanMetrics{}, err
	}

	dbPerformanceEvents, err := extractMetricsFromJSONString(escapedJSON, *query.EventID, *query.ThreadID)
	if err != nil {
		return []utils.QueryPlanMetrics{}, err
	}

	return dbPerformanceEvents, nil
}

// getQueryID extracts the query ID, returning an error if it is nil.
func getQueryID(query utils.IndividualQueryMetrics) (string, error) {
	if query.QueryID != nil {
		return *query.QueryID, nil
	}
	return "", fmt.Errorf("%w", utils.ErrQueryIDNil)
}

// getQueryText extracts and validates the query text.
func getQueryText(query utils.IndividualQueryMetrics, queryID string) (string, error) {
	if query.QueryText == nil {
		log.Warn("Query text is nil, skipping. Query ID: %s", queryID)
		return "", fmt.Errorf("%w", utils.ErrQueryTextNil)
	}

	queryText := strings.TrimSpace(*query.QueryText)
	if queryText == "" {
		log.Warn("Query text is empty, skipping. Query ID: %s", queryID)
		return "", fmt.Errorf("%w", utils.ErrQueryTextEmpty)
	}

	return queryText, nil
}

// executeExplainQuery executes the EXPLAIN query and returns the result as a JSON string.
func executeExplainQuery(ctx context.Context, db utils.DataSource, queryText string) (string, error) {
	execPlanQuery := fmt.Sprintf(constants.ExplainQueryFormat, queryText)
	rows, err := db.QueryxContext(ctx, execPlanQuery)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var execPlanJSON string
	if rows.Next() {
		err := rows.Scan(&execPlanJSON)
		if err != nil {
			return "", err
		}
	} else {
		err := fmt.Errorf("%w for query '%s'", utils.ErrNoRowsReturned, queryText)
		log.Error(err.Error())
		return "", err
	}

	return execPlanJSON, nil
}

// deduplicateJSONKeys renames duplicate keys in a JSON string by appending _N suffixes.
// MariaDB's EXPLAIN FORMAT=JSON can produce duplicate "table" and "block-nl-join" keys
// at the same object level for multi-table queries. Go's encoding/json silently drops
// all but the last occurrence. This function walks the token stream and renames duplicates
// before any json.Unmarshal call, preserving all table data.
func deduplicateJSONKeys(jsonStr string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()

	result, err := deduplicateValue(dec)
	if err != nil {
		return "", fmt.Errorf("failed to deduplicate JSON keys: %w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to re-encode deduplicated JSON: %w", err)
	}

	return string(out), nil
}

// deduplicateValue reads one JSON value (object, array, or primitive) from the decoder.
func deduplicateValue(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return deduplicateObject(dec)
		case '[':
			return deduplicateArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter: %v", v)
		}
	default:
		return v, nil
	}
}

// deduplicateObject reads an object from the decoder, renaming duplicate keys with _N suffixes.
func deduplicateObject(dec *json.Decoder) (map[string]interface{}, error) {
	keyCounts := make(map[string]int)
	result := make(map[string]interface{})

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", tok)
		}

		val, err := deduplicateValue(dec)
		if err != nil {
			return nil, err
		}

		count := keyCounts[key]
		keyCounts[key] = count + 1
		if count > 0 {
			key = fmt.Sprintf("%s_%d", key, count)
		}

		result[key] = val
	}

	// consume closing '}'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	return result, nil
}

// deduplicateArray reads an array from the decoder, recursively processing each element.
func deduplicateArray(dec *json.Decoder) ([]interface{}, error) {
	var arr []interface{}
	for dec.More() {
		val, err := deduplicateValue(dec)
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return arr, nil
}

// escapeAllStringsInJSON recursively escapes all string values in the JSON.
func escapeAllStringsInJSON(jsonString string) (string, error) {
	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonString), &jsonData)
	if err != nil {
		return "", err
	}

	escapedData := escapeJSONValue(jsonData)

	escapedJSON, err := json.Marshal(escapedData)
	if err != nil {
		return "", err
	}

	return string(escapedJSON), nil
}

// escapeJSONValue recursively traverses the JSON data and escapes all string values.
func escapeJSONValue(data interface{}) interface{} {
	switch value := data.(type) {
	case map[string]interface{}:
		for k, v := range value {
			value[k] = escapeJSONValue(v)
		}
		return value
	case []interface{}:
		for i, v := range value {
			value[i] = escapeJSONValue(v)
		}
		return value
	case string:
		return escapeString(value)
	default:
		return value
	}
}

// escapeString escapes special characters in a string.
func escapeString(str string) string {
	// Escape backslashes
	escapedStr := strings.ReplaceAll(str, "\\", "\\\\")
	// Escape double quotes
	escapedStr = strings.ReplaceAll(escapedStr, "\"", "\\\"")
	// Escape backticks
	escapedStr = strings.ReplaceAll(escapedStr, "`", "\\`")
	// Add more escaping rules here for other characters if needed
	return escapedStr
}

// extractMetricsFromJSONString extracts metrics from a JSON string.
func extractMetricsFromJSONString(jsonString string, eventID uint64, threadID uint64) ([]utils.QueryPlanMetrics, error) {
	js, err := simplejson.NewJson([]byte(jsonString))
	if err != nil {
		log.Error("Error creating simplejson from byte slice: %v", err)
		return []utils.QueryPlanMetrics{}, err
	}

	memo := utils.Memo{QueryCost: ""}
	stepID := 0
	dbPerformanceEvents := make([]utils.QueryPlanMetrics, 0)
	dbPerformanceEvents = extractMetrics(js, dbPerformanceEvents, eventID, threadID, memo, &stepID)

	return dbPerformanceEvents, nil
}

// extractMetrics recursively retrieves metrics from the query plan.
// Handles both MySQL and MariaDB JSON formats:
//   - filtered: MySQL uses string ("100.00"), MariaDB uses number (100) — try String() then Float64()
//   - rows: MySQL uses rows_examined_per_scan, MariaDB uses rows — fall back to "rows" field
//   - cost_info: MySQL-only block, gracefully returns empty on MariaDB
//   - cost: MariaDB 11.4+ flat number at query_block/table level — falls back when cost_info absent
func extractMetrics(js *simplejson.Json, dbPerformanceEvents []utils.QueryPlanMetrics, eventID uint64, threadID uint64, memo utils.Memo, stepID *int) []utils.QueryPlanMetrics {
	tableName, _ := js.Get("table_name").String()
	accessType, _ := js.Get("access_type").String()

	// Query cost: MySQL uses cost_info.query_cost (string), MariaDB 11.4+ uses flat cost (number)
	queryCost, _ := js.Get("cost_info").Get("query_cost").String()
	if queryCost == "" {
		if costVal, err := js.Get("cost").Float64(); err == nil {
			queryCost = fmt.Sprintf("%.5f", costVal)
		}
	}

	// Rows: MySQL uses rows_examined_per_scan, MariaDB uses rows.
	// Check error (not zero-value) because 0 is a valid MySQL value for const access on empty tables.
	rowsExaminedPerScan, err := js.Get("rows_examined_per_scan").Int64()
	if err != nil {
		rowsExaminedPerScan, _ = js.Get("rows").Int64()
	}

	rowsProducedPerJoin, _ := js.Get("rows_produced_per_join").Int64()

	// Filtered: MySQL uses string ("100.00"), MariaDB uses number (100)
	filtered, _ := js.Get("filtered").String()
	if filtered == "" {
		if fVal, err := js.Get("filtered").Float64(); err == nil {
			filtered = fmt.Sprintf("%.2f", fVal)
		}
	}
	readCost, _ := js.Get("cost_info").Get("read_cost").String()
	evalCost, _ := js.Get("cost_info").Get("eval_cost").String()
	prefixCost, _ := js.Get("cost_info").Get("prefix_cost").String()
	dataReadPerJoin, _ := js.Get("cost_info").Get("data_read_per_join").String()
	usingIndex, _ := js.Get("using_index").Bool()
	keyLength, _ := js.Get("key_length").String()
	possibleKeysArray, _ := js.Get("possible_keys").StringArray()
	key, _ := js.Get("key").String()
	usedKeyPartsArray, _ := js.Get("used_key_parts").StringArray()
	refArray, _ := js.Get("ref").StringArray()

	possibleKeys := strings.Join(possibleKeysArray, ",")
	usedKeyParts := strings.Join(usedKeyPartsArray, ",")
	ref := strings.Join(refArray, ",")

	if queryCost != "" {
		memo.QueryCost = queryCost
	}

	if tableName != "" || accessType != "" || rowsExaminedPerScan != 0 || rowsProducedPerJoin != 0 || filtered != "" || readCost != "" || evalCost != "" {
		dbPerformanceEvents = append(dbPerformanceEvents, utils.QueryPlanMetrics{
			EventID:             eventID,
			ThreadID:            threadID,
			QueryCost:           memo.QueryCost,
			StepID:              *stepID,
			TableName:           tableName,
			AccessType:          accessType,
			RowsExaminedPerScan: rowsExaminedPerScan,
			RowsProducedPerJoin: rowsProducedPerJoin,
			Filtered:            filtered,
			ReadCost:            readCost,
			EvalCost:            evalCost,
			PossibleKeys:        possibleKeys,
			Key:                 key,
			UsedKeyParts:        usedKeyParts,
			Ref:                 ref,
			PrefixCost:          prefixCost,
			DataReadPerJoin:     dataReadPerJoin,
			UsingIndex:          fmt.Sprintf("%t", usingIndex),
			KeyLength:           keyLength,
		})
		*stepID++
	}

	if jsMap, _ := js.Map(); jsMap != nil {
		dbPerformanceEvents = processMap(jsMap, dbPerformanceEvents, eventID, threadID, memo, stepID)
	}

	return dbPerformanceEvents
}

// processMap processes a map within the JSON object.
func processMap(jsMap map[string]interface{}, dbPerformanceEvents []utils.QueryPlanMetrics, eventID uint64, threadID uint64, memo utils.Memo, stepID *int) []utils.QueryPlanMetrics {
	for _, value := range jsMap {
		if value != nil {
			t := reflect.TypeOf(value)
			if t.Kind() == reflect.Map {
				dbPerformanceEvents = processMapValue(value, dbPerformanceEvents, eventID, threadID, memo, stepID)
			} else if t.Kind() == reflect.Slice {
				dbPerformanceEvents = processSliceValue(value, dbPerformanceEvents, eventID, threadID, memo, stepID)
			}
		}
	}
	return dbPerformanceEvents
}

// processMapValue processes a map value within the JSON object.
func processMapValue(value interface{}, dbPerformanceEvents []utils.QueryPlanMetrics, eventID uint64, threadID uint64, memo utils.Memo, stepID *int) []utils.QueryPlanMetrics {
	if t := reflect.TypeOf(value); t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.Interface {
		jsBytes, err := json.Marshal(value)
		if err != nil {
			log.Error("Error marshaling map: %v", err)
		}

		convertedSimpleJSON, err := simplejson.NewJson(jsBytes)
		if err != nil {
			log.Error("Error creating simplejson from byte slice: %v", err)
		}

		dbPerformanceEvents = extractMetrics(convertedSimpleJSON, dbPerformanceEvents, eventID, threadID, memo, stepID)
	}
	return dbPerformanceEvents
}

// processSliceValue processes a slice value within the JSON object.
func processSliceValue(value interface{}, dbPerformanceEvents []utils.QueryPlanMetrics, eventID uint64, threadID uint64, memo utils.Memo, stepID *int) []utils.QueryPlanMetrics {
	for _, element := range value.([]interface{}) {
		if elementJSON, ok := element.(map[string]interface{}); ok {
			jsBytes, err := json.Marshal(elementJSON)
			if err != nil {
				log.Error("Error marshaling map: %v", err)
			}

			convertedSimpleJSON, err := simplejson.NewJson(jsBytes)
			if err != nil {
				log.Error("Error creating simplejson from byte slice: %v", err)
			}

			dbPerformanceEvents = extractMetrics(convertedSimpleJSON, dbPerformanceEvents, eventID, threadID, memo, stepID)
		}
	}
	return dbPerformanceEvents
}

// SetExecutionPlanMetrics sets the execution plan metrics.
func SetExecutionPlanMetrics(i *integration.Integration, args arguments.ArgumentList, metrics []utils.QueryPlanMetrics) error {
	// Pre-allocate the slice with the length of the metrics slice
	metricList := make([]interface{}, 0, len(metrics))
	for _, metricData := range metrics {
		metricList = append(metricList, metricData)
	}

	err := utils.IngestMetric(metricList, "MysqlQueryExecutionSample", i, args)
	if err != nil {
		log.Error("Error setting execution plan metrics: %v", err)
		return err
	}
	return nil
}

// isSupportedStatement checks if the given query is a supported statement.
func isSupportedStatement(query string) bool {
	upperCaseQuery := strings.ToUpper(strings.TrimSpace(query))
	/*
		SupportedStatements defines the SQL statements for which this integration fetches query execution plans.
		Restricting the supported statements improves compatibility and reduces the complexity of plan analysis.
	*/
	supportedStatements := []string{"SELECT", "WITH"}
	for _, stmt := range supportedStatements {
		if strings.HasPrefix(upperCaseQuery, stmt) {
			return true
		}
	}
	return false
}
