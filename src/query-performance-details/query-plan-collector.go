package query_performance_details

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

type QueryPlanMetrics struct {
	QueryID             string `json:"query_id" db:"query_id"`
	AnonymizedQueryText string `json:"query_text" db:"query_text"`
	QueryText           string `json:"query_sample_text" db:"query_sample_text"`
}

type TableMetrics struct {
	StepID        int     `json:"step_id"`
	ExecutionStep string  `json:"Execution Step"`
	AccessType    string  `json:"access_type"`
	RowsExamined  int64   `json:"rows_examined"`
	RowsProduced  int64   `json:"rows_produced"`
	Filtered      float64 `json:"filtered (%)"`
	ReadCost      float64 `json:"read_cost"`
	EvalCost      float64 `json:"eval_cost"`
	DataRead      float64 `json:"data_read"`
	ExtraInfo     string  `json:"extra_info"`
}

type ExecutionPlan struct {
	TableMetrics []TableMetrics `json:"table_metrics"`
	TotalCost    float64        `json:"total_cost"`
}

func currentQueryMetrics(db dataSource, QueryIDList []string) ([]QueryPlanMetrics, error) {
	// Check Performance Schema availability
	metrics, err := collectCurrentQueryMetrics(db, QueryIDList)
	if err != nil {
		log.Error("Failed to collect query metrics: %v", err)
		return nil, err
	}

	return metrics, nil
}

func recentQueryMetrics(db dataSource, QueryIDList []string) ([]QueryPlanMetrics, error) {
	// Check Performance Schema availability
	metrics, err := collectRecentQueryMetrics(db, QueryIDList)
	if err != nil {
		log.Error("Failed to collect query metrics: %v", err)
		return nil, err
	}

	return metrics, nil
}

func extensiveQueryMetrics(db dataSource, QueryIDList []string) ([]QueryPlanMetrics, error) {
	// Check Performance Schema availability
	metrics, err := collectExtensiveQueryMetrics(db, QueryIDList)
	if err != nil {
		log.Error("Failed to collect query metrics: %v", err)
		return nil, err
	}

	return metrics, nil
}

func collectCurrentQueryMetrics(db dataSource, queryIDList []string) ([]QueryPlanMetrics, error) {
	if len(queryIDList) == 0 {
		log.Warn("queryIDList is empty")
		return nil, nil
	}
	// Building the placeholder string for the IN clause
	placeholders := make([]string, len(queryIDList))
	for i := range queryIDList {
		placeholders[i] = "?"
	}

	// Joining the placeholders to form the IN clause
	inClause := strings.Join(placeholders, ", ")

	// Creating the query string with the IN clause
	query := fmt.Sprintf(`
		SELECT
			DIGEST AS query_id,
			DIGEST_TEXT AS query_text,
			SQL_TEXT AS query_sample_text
		FROM performance_schema.events_statements_current
		WHERE DIGEST IN (%s)
			AND CURRENT_SCHEMA NOT IN ('', 'mysql', 'performance_schema', 'information_schema', 'sys')
            AND SQL_TEXT NOT LIKE '%%SET %%'
            AND SQL_TEXT NOT LIKE '%%SHOW %%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%mysql%%'
            AND SQL_TEXT NOT LIKE 'EXPLAIN %%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
		ORDER BY TIMER_WAIT DESC;
	`, inClause)

	// Converting the slice of queryIDs to a slice of interface{} for db.QueryxContext
	args := make([]interface{}, len(queryIDList))
	for i, id := range queryIDList {
		args[i] = id
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()
	// fmt.Println("Current------", query, args)
	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
		// fmt.Println("Current Metric------", metric)
		metrics = append(metrics, metric)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		log.Error("Error iterating over query metrics rows: %v", err)
		return nil, err
	}

	return metrics, nil
}

func collectRecentQueryMetrics(db dataSource, queryIDList []string) ([]QueryPlanMetrics, error) {
	if len(queryIDList) == 0 {
		log.Warn("queryIDList is empty")
		return nil, nil
	}
	// Building the placeholder string for the IN clause
	placeholders := make([]string, len(queryIDList))
	for i := range queryIDList {
		placeholders[i] = "?"
	}

	// Joining the placeholders to form the IN clause
	inClause := strings.Join(placeholders, ", ")

	// Creating the query string with the IN clause
	query := fmt.Sprintf(`
		SELECT
			DIGEST AS query_id,
			DIGEST_TEXT AS query_text,
			SQL_TEXT AS query_sample_text
		FROM performance_schema.events_statements_history
		WHERE DIGEST IN (%s)
			AND CURRENT_SCHEMA NOT IN ('', 'mysql', 'performance_schema', 'information_schema', 'sys')
            AND SQL_TEXT NOT LIKE '%%SET %%'
            AND SQL_TEXT NOT LIKE '%%SHOW %%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%mysql%%'
            AND SQL_TEXT NOT LIKE 'EXPLAIN %%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
		ORDER BY TIMER_WAIT DESC;
	`, inClause)

	// Converting the slice of queryIDs to a slice of interface{} for db.QueryxContext
	args := make([]interface{}, len(queryIDList))
	for i, id := range queryIDList {
		args[i] = id
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()
	// fmt.Println("Recent------", query, args)
	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
		// fmt.Println("Recent Metric------", metric)
		metrics = append(metrics, metric)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		log.Error("Error iterating over query metrics rows: %v", err)
		return nil, err
	}

	return metrics, nil
}

func collectExtensiveQueryMetrics(db dataSource, queryIDList []string) ([]QueryPlanMetrics, error) {
	if len(queryIDList) == 0 {
		log.Warn("queryIDList is empty")
		return nil, nil
	}
	// Building the placeholder string for the IN clause
	placeholders := make([]string, len(queryIDList))
	for i := range queryIDList {
		placeholders[i] = "?"
	}

	// Joining the placeholders to form the IN clause
	inClause := strings.Join(placeholders, ", ")

	// Creating the query string with the IN clause
	query := fmt.Sprintf(`
		SELECT
			DIGEST AS query_id,
			DIGEST_TEXT AS query_text,
			SQL_TEXT AS query_sample_text
		FROM performance_schema.events_statements_history_long
		WHERE DIGEST IN (%s)
			AND CURRENT_SCHEMA NOT IN ('', 'mysql', 'performance_schema', 'information_schema', 'sys')
            AND SQL_TEXT NOT LIKE '%%SET %%'
            AND SQL_TEXT NOT LIKE '%%SHOW %%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%mysql%%'
            AND SQL_TEXT NOT LIKE 'EXPLAIN %%'
            AND SQL_TEXT NOT LIKE '%%PERFORMANCE_SCHEMA%%'
            AND SQL_TEXT NOT LIKE '%%INFORMATION_SCHEMA%%'
		ORDER BY TIMER_WAIT DESC;
	`, inClause)

	// Converting the slice of queryIDs to a slice of interface{} for db.QueryxContext
	args := make([]interface{}, len(queryIDList))
	for i, id := range queryIDList {
		args[i] = id
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()
	// fmt.Println("Extensive------", query, args)
	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
		// fmt.Println("Extensive Metric------", metric)
		metrics = append(metrics, metric)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		log.Error("Error iterating over query metrics rows: %v", err)
		return nil, err
	}

	return metrics, nil
}

func extractTableMetrics(tableInfo map[string]interface{}, stepID int) ([]TableMetrics, int) {
	var tableMetrics []TableMetrics
	stepID++

	if table, exists := tableInfo["table"].(map[string]interface{}); exists {
		metrics := TableMetrics{
			StepID:        stepID,
			ExecutionStep: getString(table, "table_name"),
			AccessType:    getString(table, "access_type"),
			RowsExamined:  getInt64(table, "rows_examined_per_scan"),
			RowsProduced:  getInt64(table, "rows_produced_per_join"),
			Filtered:      getFloat64(table, "filtered"),
		}

		if costInfo, ok := table["cost_info"].(map[string]interface{}); ok {
			metrics.ReadCost = getFloat64(costInfo, "read_cost")
			metrics.EvalCost = getFloat64(costInfo, "eval_cost")
			metrics.DataRead = getFloat64(costInfo, "data_read_per_join")
		}

		if usedKeyParts, ok := table["used_key_parts"].([]interface{}); ok {
			metrics.ExtraInfo = convertToStringArray(usedKeyParts)
		}

		tableMetrics = append(tableMetrics, metrics)
	}

	if nestedLoop, exists := tableInfo["nested_loop"].([]interface{}); exists {
		for _, nested := range nestedLoop {
			if nestedMap, ok := nested.(map[string]interface{}); ok {
				metrics, newStepID := extractTableMetrics(nestedMap, stepID)
				tableMetrics = append(tableMetrics, metrics...)
				stepID = newStepID
			} else {
				log.Error("Unexpected type for nested element: %T", nested)
			}
		}
	}

	return tableMetrics, stepID
}

func extractMetricsFromPlan(plan map[string]interface{}) ExecutionPlan {
	var metrics ExecutionPlan
	queryBlock, _ := plan["query_block"].(map[string]interface{})
	stepID := 0
	// fmt.Println("Query Plan------", plan)

	// Handle cost_info safely
	if costInfo, exists := queryBlock["cost_info"].(map[string]interface{}); exists {
		metrics.TotalCost = getCostSafely(costInfo, "query_cost")
	}

	// Handle nested_loop safely
	if nestedLoop, exists := queryBlock["nested_loop"].([]interface{}); exists {
		for _, nested := range nestedLoop {
			if nestedMap, ok := nested.(map[string]interface{}); ok {
				nestedMetrics, newStepID := extractTableMetrics(nestedMap, stepID)
				metrics.TableMetrics = append(metrics.TableMetrics, nestedMetrics...)
				stepID = newStepID
			} else {
				log.Error("Unexpected type for nested element: %T", nested)
			}
		}
	}

	// Handle ordering_operation and nested grouping_operation
	if orderingOp, exists := queryBlock["ordering_operation"].(map[string]interface{}); exists {
		if groupingOp, exists := orderingOp["grouping_operation"].(map[string]interface{}); exists {
			if nestedLoop, exists := groupingOp["nested_loop"].([]interface{}); exists {
				for _, nested := range nestedLoop {
					if nestedMap, ok := nested.(map[string]interface{}); ok {
						nestedMetrics, newStepID := extractTableMetrics(nestedMap, stepID)
						metrics.TableMetrics = append(metrics.TableMetrics, nestedMetrics...)
						stepID = newStepID
					} else {
						log.Error("Unexpected type for nested element in grouping_operation: %T", nested)
					}
				}
			}
		}
	}

	// Handle table entry safely
	if table, exists := queryBlock["table"].(map[string]interface{}); exists {
		metricsTable, _ := extractTableMetrics(map[string]interface{}{"table": table}, stepID)
		metrics.TableMetrics = append(metrics.TableMetrics, metricsTable...)
	}

	return metrics
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
		// Log unexpected types
		log.Error("Unexpected type for %q: %T", key, val)
	}
	return "" // Default to empty string if nil or type doesn't match
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case string:
			parsedVal, err := parseSpecialFloat(v)
			if err == nil {
				return parsedVal
			}
			log.Error("Failed to parse string to float64 for key %q: %v", key, err)
		default:
			log.Error("Unhandled type for key %q: %T", key, val)
		}
	}
	return 0.0 // Default to 0.0 if nil or type doesn't match
}

func getInt64(m map[string]interface{}, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case string:
			parsedVal, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				return parsedVal
			}
			log.Error("Failed to parse string to int64 for key %q: %v", key, err)
		default:
			log.Error("Unhandled type for key %q: %T", key, val)
		}
	}
	return 0 // Default to 0 if nil or type doesn't match
}

func convertToStringArray(arr []interface{}) string {
	parts := make([]string, len(arr))
	for i, v := range arr {
		if str, ok := v.(string); ok {
			parts[i] = str
		} else {
			log.Error("Unexpected type in array at index %d: %T", i, v)
		}
	}
	return strings.Join(parts, ", ")
}

func getCostSafely(costInfo map[string]interface{}, key string) float64 {
	if costValue, ok := costInfo[key]; ok {
		switch v := costValue.(type) {
		case float64:
			return v
		case string:
			parsedVal, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsedVal
			}
			log.Error("Failed to parse string to float64 for key %q: %v", key, err)
		default:
			log.Error("Unhandled type for key %q: %T", key, costValue)
		}
	}
	return 0.0 // Default to 0.0 if key doesn't exist or type doesn't match
}

func getStringValueSafe(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return ""
	default:
		log.Error("Unexpected type for value: %T", value)
		return ""
	}
}

func parseSpecialFloat(value string) (float64, error) {
	multipliers := map[string]float64{
		"K": 1e3,
		"M": 1e6,
		"G": 1e9,
		"T": 1e12,
	}

	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(value, suffix) {
			baseValue := strings.TrimSuffix(value, suffix)
			parsedVal, err := strconv.ParseFloat(baseValue, 64)
			if err != nil {
				return 0, err
			}
			return parsedVal * multiplier, nil
		}
	}

	return strconv.ParseFloat(value, 64)
}

func getFloat64ValueSafe(value interface{}) float64 {
	if value == nil {
		return 0.0
	}
	switch v := value.(type) {
	case float64:
		return v
	case string:
		parsedVal, err := parseSpecialFloat(v)
		if err == nil {
			return parsedVal
		}
		log.Error("Failed to parse string to float64: %v", err)
	case sql.NullString:
		if v.Valid {
			parsedVal, err := parseSpecialFloat(v.String)
			if err == nil {
				return parsedVal
			}
			log.Error("Failed to parse sql.NullString to float64: %v", err)
		}
	default:
		log.Error("Unexpected type for value: %T", value)
	}
	return 0.0
}

func getInt64ValueSafe(value interface{}) int64 {
	if value == nil {
		return 0
	}
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsedVal, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return parsedVal
		}
		log.Error("Failed to parse string to int64: %v", err)
	case sql.NullString:
		if v.Valid {
			parsedVal, err := strconv.ParseInt(v.String, 10, 64)
			if err == nil {
				return parsedVal
			}
			log.Error("Failed to parse sql.NullString to int64: %v", err)
		}
	default:
		log.Error("Unexpected type for value: %T", value)
	}
	return 0
}

func formatAsTable(metrics []TableMetrics) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"step_id", "Execution Step", "access_type", "rows_examined", "rows_produced", "filtered (%)", "read_cost", "eval_cost", "data_read", "extra_info"})

	for _, metric := range metrics {
		row := []string{
			fmt.Sprintf("%d", metric.StepID),
			metric.ExecutionStep,
			metric.AccessType,
			fmt.Sprintf("%d", metric.RowsExamined),
			fmt.Sprintf("%d", metric.RowsProduced),
			fmt.Sprintf("%.2f", metric.Filtered),
			fmt.Sprintf("%.2f", metric.ReadCost),
			fmt.Sprintf("%.2f", metric.EvalCost),
			fmt.Sprintf("%.2f", metric.DataRead),
			metric.ExtraInfo,
		}
		table.Append(row)
	}

	table.Render()
}

func captureExecutionPlans(db dataSource, queries []QueryPlanMetrics) ([]map[string]interface{}, error) {
	supportedStatements := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}
	var events []map[string]interface{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, query := range queries {
		queryText := strings.TrimSpace(query.QueryText)
		upperQueryText := strings.ToUpper(queryText)

		if !supportedStatements[strings.Split(upperQueryText, " ")[0]] {
			fmt.Printf("Skipping unsupported query for EXPLAIN: %s\n", queryText)
			continue
		}

		if strings.Contains(queryText, "?") {
			fmt.Printf("Skipping query with placeholders for EXPLAIN: %s\n", queryText)
			continue
		}

		execPlanQuery := fmt.Sprintf("EXPLAIN FORMAT=JSON %s", queryText)
		rows, err := db.QueryxContext(ctx, execPlanQuery)
		if err != nil {
			log.Error("Error executing EXPLAIN for query '%s': %v\n", queryText, err)
			continue
		}
		defer rows.Close()

		var execPlanJSON string
		if rows.Next() {
			err := rows.Scan(&execPlanJSON)
			if err != nil {
				log.Error("Failed to scan execution plan: %v", err)
				continue
			}
		}

		var execPlan map[string]interface{}
		err = json.Unmarshal([]byte(execPlanJSON), &execPlan)
		if err != nil {
			log.Error("Failed to unmarshal execution plan: %v", err)
			continue
		}
		// fmt.Println("Query execPlan------", execPlan)
		metrics := extractMetricsFromPlan(execPlan)

		baseIngestionData := map[string]interface{}{
			"eventType":  "MySQLExecutionPlan",
			"query_id":   query.QueryID,
			"query_text": query.AnonymizedQueryText,
			"total_cost": metrics.TotalCost,
			"step_id":    0,
		}

		events = append(events, baseIngestionData)
		formatAsTable(metrics.TableMetrics)

		for _, metric := range metrics.TableMetrics {
			tableIngestionData := make(map[string]interface{})
			for k, v := range baseIngestionData {
				tableIngestionData[k] = v
			}
			tableIngestionData["step_id"] = metric.StepID
			tableIngestionData["Execution Step"] = metric.ExecutionStep
			tableIngestionData["access_type"] = metric.AccessType
			tableIngestionData["rows_examined"] = metric.RowsExamined
			tableIngestionData["rows_produced"] = metric.RowsProduced
			tableIngestionData["filtered (%)"] = metric.Filtered
			tableIngestionData["read_cost"] = metric.ReadCost
			tableIngestionData["eval_cost"] = metric.EvalCost
			tableIngestionData["data_read"] = metric.DataRead
			tableIngestionData["extra_info"] = metric.ExtraInfo

			events = append(events, tableIngestionData)
		}
	}

	if len(events) == 0 {
		return []map[string]interface{}{}, nil
	}

	return events, nil
}

func populateQueryPlanMetrics(e *integration.Entity, args arguments.ArgumentList, metrics []map[string]interface{}) error {
	for _, metricObject := range metrics {
		// Create a new metric set for each row
		ms := createMetricSet(e, "MysqlQueryPlan", args)
		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{
			"query_id":       {getStringValueSafe(metricObject["query_id"]), metric.ATTRIBUTE},
			"query_text":     {getStringValueSafe(metricObject["query_text"]), metric.ATTRIBUTE},
			"total_cost":     {getFloat64ValueSafe(metricObject["total_cost"]), metric.GAUGE},
			"step_id":        {getInt64ValueSafe(metricObject["step_id"]), metric.GAUGE},
			"Execution Step": {getStringValueSafe(metricObject["Execution Step"]), metric.ATTRIBUTE},
			"access_type":    {getStringValueSafe(metricObject["access_type"]), metric.ATTRIBUTE},
			"rows_examined":  {getInt64ValueSafe(metricObject["rows_examined"]), metric.GAUGE},
			"rows_produced":  {getInt64ValueSafe(metricObject["rows_produced"]), metric.GAUGE},
			"filtered (%)":   {getFloat64ValueSafe(metricObject["filtered (%)"]), metric.GAUGE},
			"read_cost":      {getFloat64ValueSafe(metricObject["read_cost"]), metric.GAUGE},
			"eval_cost":      {getFloat64ValueSafe(metricObject["eval_cost"]), metric.GAUGE},
			"data_read":      {getFloat64ValueSafe(metricObject["data_read"]), metric.GAUGE},
			"extra_info":     {getStringValueSafe(metricObject["extra_info"]), metric.ATTRIBUTE},
		}

		for name, metricData := range metricsMap {
			err := ms.SetMetric(name, metricData.Value, metricData.MetricType)
			if err != nil {
				log.Error("Error setting value for %s: %v", name, err)
				continue
			}
		}

		// Print the metric set for debugging
		printMetricSet(ms)
	}

	return nil
}

func printMetricSet(ms *metric.Set) {
	fmt.Println("Metric Set Contents:")
	for name, metric := range ms.Metrics {
		fmt.Printf("Name: %s, Value: %v, Type: %v\n", name, metric, "unknown")
	}
}

func populateQueryMetrics(e *integration.Entity, args arguments.ArgumentList, metrics []QueryPlanMetrics) error {
	for _, metricObject := range metrics {

		// Create a new metric set for each row
		ms := createMetricSet(e, "MysqlIndividualQueries", args)

		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{

			"query_id":   {metricObject.QueryID, metric.ATTRIBUTE},
			"query_text": {metricObject.AnonymizedQueryText, metric.ATTRIBUTE},
		}

		for name, metric := range metricsMap {
			err := ms.SetMetric(name, metric.Value, metric.MetricType)
			if err != nil {
				log.Warn("Error setting value:  %s", err)
				continue
			}
		}
	}
	return nil
}
