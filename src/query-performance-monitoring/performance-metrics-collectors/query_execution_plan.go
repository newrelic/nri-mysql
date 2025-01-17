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
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// PopulateExecutionPlans populates execution plans for the given queries.
func PopulateExecutionPlans(db utils.DataSource, queryGroups []utils.QueryGroup, i *integration.Integration, e *integration.Entity, args arguments.ArgumentList) {
	var events []utils.QueryPlanMetrics

	for _, group := range queryGroups {
		dsn := utils.GenerateDSN(args, group.Database)
		// Open the DB connection
		db, err := utils.OpenDB(dsn)
		utils.FatalIfErr(err)
		defer db.Close()

		for _, query := range group.Queries {
			tableIngestionDataList, err := processExecutionPlanMetrics(db, query)
			if err != nil {
				log.Error("Error processing execution plan metrics: %v", err)
			}
			events = append(events, tableIngestionDataList...)
		}
	}

	// Return if no metrics are collected
	if len(events) == 0 {
		return
	}

	err := SetExecutionPlanMetrics(i, args, events)
	if err != nil {
		log.Error("Error publishing execution plan metrics: %v", err)
	}
}

// processExecutionPlanMetrics processes the execution plan metrics for a given query.
func processExecutionPlanMetrics(db utils.DataSource, query utils.IndividualQueryMetrics) ([]utils.QueryPlanMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QueryPlanTimeoutDuration)
	defer cancel()

	if query.QueryText == nil || strings.TrimSpace(*query.QueryText) == "" {
		log.Warn("Query text is empty or nil, skipping.")
		return []utils.QueryPlanMetrics{}, nil
	}
	queryText := strings.TrimSpace(*query.QueryText)
	upperQueryText := strings.ToUpper(queryText)

	// Check if the query is a supported statement
	if !isSupportedStatement(upperQueryText) {
		log.Warn("Skipping unsupported query for EXPLAIN: %s", queryText)
		return []utils.QueryPlanMetrics{}, nil
	}

	// Skip queries with placeholders
	if strings.Contains(queryText, "?") {
		log.Warn("Skipping query with placeholders for EXPLAIN: %s", queryText)
		return []utils.QueryPlanMetrics{}, nil
	}

	// Execute the EXPLAIN query
	execPlanQuery := fmt.Sprintf(constants.ExplainQueryFormat, queryText)
	rows, err := db.QueryxContext(ctx, execPlanQuery)
	if err != nil {
		return []utils.QueryPlanMetrics{}, err
	}
	defer rows.Close()

	var execPlanJSON string
	if rows.Next() {
		err := rows.Scan(&execPlanJSON)
		if err != nil {
			return []utils.QueryPlanMetrics{}, err
		}
	} else {
		log.Error("No rows returned from EXPLAIN for query '%s'", queryText)
		return []utils.QueryPlanMetrics{}, nil
	}

	// Extract metrics from the JSON string
	dbPerformanceEvents, err := extractMetricsFromJSONString(execPlanJSON, *query.EventID, *query.ThreadID)
	if err != nil {
		return []utils.QueryPlanMetrics{}, err
	}

	return dbPerformanceEvents, nil
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
func extractMetrics(js *simplejson.Json, dbPerformanceEvents []utils.QueryPlanMetrics, eventID uint64, threadID uint64, memo utils.Memo, stepID *int) []utils.QueryPlanMetrics {
	tableName, _ := js.Get("table_name").String()
	queryCost, _ := js.Get("cost_info").Get("query_cost").String()
	accessType, _ := js.Get("access_type").String()
	rowsExaminedPerScan, _ := js.Get("rows_examined_per_scan").Int64()
	rowsProducedPerJoin, _ := js.Get("rows_produced_per_join").Int64()
	filtered, _ := js.Get("filtered").String()
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
	insert, _ := js.Get("insert").Bool()
	update, _ := js.Get("update").Bool()
	delete, _ := js.Get("delete").Bool()

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
			InsertOperation:     fmt.Sprintf("%t", insert),
			UpdateOperation:     fmt.Sprintf("%t", update),
			DeleteOperation:     fmt.Sprintf("%t", delete),
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
	for _, stmt := range strings.Split(constants.SupportedStatements, " ") {
		if strings.HasPrefix(query, stmt) {
			return true
		}
	}
	return false
}
