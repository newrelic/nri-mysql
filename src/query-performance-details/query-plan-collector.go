package query_performance_details

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

type QueryPlanMetrics struct {
	QueryID   string `json:"query_id" db:"query_id"`
	QueryText string `json:"query_text" db:"query_text"`
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
			DIGEST_TEXT AS query_text
		FROM performance_schema.events_statements_current
		WHERE DIGEST IN (%s)
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
	fmt.Println("Current------", query, args)
	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
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
			DIGEST_TEXT AS query_text
		FROM performance_schema.events_statements_current
		WHERE DIGEST IN (%s)
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
	fmt.Println("Recent------", query, args)
	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
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
			DIGEST_TEXT AS query_text
		FROM performance_schema.events_statements_current
		WHERE DIGEST IN (%s)
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
	fmt.Println("Extensive------", query, args)
	// Execute the query using QueryxContext
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Slice to hold the metrics
	var metrics []QueryPlanMetrics

	// Iterate over the rows and scan into the metrics slice
	for rows.Next() {
		var metric QueryPlanMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
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

	if table, exists := tableInfo["table"]; exists {
		tableMap := table.(map[string]interface{})
		metrics := TableMetrics{
			StepID:        stepID,
			ExecutionStep: tableMap["table_name"].(string),
			AccessType:    tableMap["access_type"].(string),
			RowsExamined:  int64(tableMap["rows_examined_per_scan"].(float64)),
			RowsProduced:  int64(tableMap["rows_produced_per_join"].(float64)),
			Filtered:      tableMap["filtered"].(float64),
		}

		if costInfo, ok := tableMap["cost_info"].(map[string]interface{}); ok {
			metrics.ReadCost = costInfo["read_cost"].(float64)
			metrics.EvalCost = costInfo["eval_cost"].(float64)
			metrics.DataRead = costInfo["data_read_per_join"].(float64)
		}

		if usedKeyParts, ok := tableMap["used_key_parts"].([]interface{}); ok {
			parts := make([]string, len(usedKeyParts))
			for i, part := range usedKeyParts {
				parts[i] = part.(string)
			}
			metrics.ExtraInfo = strings.Join(parts, ", ")
		}

		tableMetrics = append(tableMetrics, metrics)
	}

	if nestedLoop, exists := tableInfo["nested_loop"].([]interface{}); exists {
		for _, nested := range nestedLoop {
			metrics, newStepID := extractTableMetrics(nested.(map[string]interface{}), stepID)
			tableMetrics = append(tableMetrics, metrics...)
			stepID = newStepID
		}
	}

	return tableMetrics, stepID
}

func extractMetricsFromPlan(plan map[string]interface{}) ExecutionPlan {
	var metrics ExecutionPlan
	queryBlock, _ := plan["query_block"].(map[string]interface{})
	stepID := 0

	if costInfo, exists := queryBlock["cost_info"].(map[string]interface{}); exists {
		metrics.TotalCost = costInfo["query_cost"].(float64)
	}

	if nestedLoop, exists := queryBlock["nested_loop"].([]interface{}); exists {
		for _, nested := range nestedLoop {
			nestedMetrics, newStepID := extractTableMetrics(nested.(map[string]interface{}), stepID)
			metrics.TableMetrics = append(metrics.TableMetrics, nestedMetrics...)
			stepID = newStepID
		}
	}

	if table, exists := queryBlock["table"].(map[string]interface{}); exists {
		metricsTable, _ := extractTableMetrics(map[string]interface{}{"table": table}, stepID)
		metrics.TableMetrics = append(metrics.TableMetrics, metricsTable...)
	}

	return metrics
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

		metrics := extractMetricsFromPlan(execPlan)

		baseIngestionData := map[string]interface{}{
			"eventType":  "MySQLExecutionPlan",
			"query_id":   query.QueryID,
			"query_text": query.QueryText,
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

func populateQueryPlanMetrics(ms *metric.Set, metrics []map[string]interface{}) error {
	for _, metricObject := range metrics {
		if ms == nil {
			return fmt.Errorf("failed to create metric set")
		}

		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{
			"query_id":       {metricObject["query_id"], metric.ATTRIBUTE},
			"query_text":     {getStringValue(sql.NullString{String: metricObject["query_text"].(string), Valid: true}), metric.ATTRIBUTE},
			"database_name":  {getStringValue(sql.NullString{String: metricObject["database_name"].(string), Valid: true}), metric.ATTRIBUTE},
			"total_cost":     {metricObject["total_cost"], metric.GAUGE},
			"step_id":        {metricObject["step_id"], metric.GAUGE},
			"Execution Step": {metricObject["Execution Step"], metric.ATTRIBUTE},
			"access_type":    {metricObject["access_type"], metric.ATTRIBUTE},
			"rows_examined":  {metricObject["rows_examined"], metric.GAUGE},
			"rows_produced":  {metricObject["rows_produced"], metric.GAUGE},
			"filtered (%)":   {metricObject["filtered (%)"], metric.GAUGE},
			"read_cost":      {metricObject["read_cost"], metric.GAUGE},
			"eval_cost":      {metricObject["eval_cost"], metric.GAUGE},
			"data_read":      {metricObject["data_read"], metric.GAUGE},
			"extra_info":     {getStringValue(sql.NullString{String: metricObject["extra_info"].(string), Valid: true}), metric.ATTRIBUTE},
		}

		for name, metricData := range metricsMap {
			err := ms.SetMetric(name, metricData.Value, metricData.MetricType)
			if err != nil {
				log.Error("Error setting value for %s: %v", name, err)
				continue
			}
		}
	}
	return nil
}

func populateQueryMetrics(ms *metric.Set, metrics []QueryPlanMetrics) error {
	for _, metricObject := range metrics {
		if ms == nil {
			return fmt.Errorf("failed to create metric set")
		}

		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{

			"query_id":   {metricObject.QueryID, metric.ATTRIBUTE},
			"query_text": {metricObject.QueryText, metric.ATTRIBUTE},
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
