package query_performance_details

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

type QueryMetrics struct {
	DBQueryID           string         `json:"db_query_id"`
	QueryID             string         `json:"query_id"`
	QueryText           sql.NullString `json:"query_text"`
	DatabaseName        sql.NullString `json:"database_name"`
	SchemaName          string         `json:"schema_name"`
	ExecutionCount      uint64         `json:"execution_count"`
	AvgCPUTimeMs        float64        `json:"avg_cpu_time_ms"`
	AvgElapsedTimeMs    float64        `json:"avg_elapsed_time_ms"`
	AvgDiskReads        float64        `json:"avg_disk_reads"`
	AvgDiskWrites       float64        `json:"avg_disk_writes"`
	HasFullTableScan    string         `json:"has_full_table_scan"`
	StatementType       string         `json:"statement_type"`
	CollectionTimestamp string         `json:"collection_timestamp"`
}

func collectQueryMetrics(db dataSource) ([]QueryMetrics, error) {
	// Check Performance Schema availability
	metrics, err := collectPerformanceSchemaMetrics(db)
	if err != nil {
		log.Error("Failed to collect query metrics: %v", err)
		return nil, err
	}

	return metrics, nil
}

func collectPerformanceSchemaMetrics(db dataSource) ([]QueryMetrics, error) {
	query := `
        SELECT
			DIGEST AS db_query_id,
            LEFT(UPPER(SHA2(DIGEST_TEXT, 256)), 16) AS query_id,
            DIGEST_TEXT AS query_text,
            SCHEMA_NAME AS database_name,
            'N/A' AS schema_name,
            COUNT_STAR AS execution_count,
            ROUND((SUM_CPU_TIME / COUNT_STAR) / 1000000000000, 3) AS avg_cpu_time_ms,
            ROUND((SUM_TIMER_WAIT / COUNT_STAR) / 1000000000000, 3) AS avg_elapsed_time_ms,
            SUM_ROWS_EXAMINED / COUNT_STAR AS avg_disk_reads,
            SUM_ROWS_AFFECTED / COUNT_STAR AS avg_disk_writes,
            CASE
                WHEN SUM_NO_INDEX_USED > 0 THEN 'Yes'
                ELSE 'No'
            END AS has_full_table_scan,
            CASE
                WHEN DIGEST_TEXT LIKE 'SELECT%' THEN 'SELECT'
                WHEN DIGEST_TEXT LIKE 'INSERT%' THEN 'INSERT'
                WHEN DIGEST_TEXT LIKE 'UPDATE%' THEN 'UPDATE'
                WHEN DIGEST_TEXT LIKE 'DELETE%' THEN 'DELETE'
                ELSE 'OTHER'
            END AS statement_type,
            DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
        FROM performance_schema.events_statements_summary_by_digest
        WHERE LAST_SEEN >= UTC_TIMESTAMP() - INTERVAL 30 SECOND
        ORDER BY avg_elapsed_time_ms DESC;
    `

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryxContext(ctx, query)
	if err != nil {
		log.Error("Failed to collect query metrics from Performance Schema: %v", err)
		return nil, err
	}
	defer rows.Close()

	var metrics []QueryMetrics
	for rows.Next() {
		var metric QueryMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan query metrics row: %v", err)
			return nil, err
		}
		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		log.Error("Error iterating over query metrics rows: %v", err)
		return nil, err
	}

	return metrics, nil
}

func populateMetrics(ms *metric.Set, metrics []QueryMetrics) error {
	for _, metricObject := range metrics {
		if ms == nil {
			return fmt.Errorf("failed to create metric set")
		}

		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{
			"db_query_id":          {metricObject.DBQueryID, metric.ATTRIBUTE},
			"query_id":             {metricObject.QueryID, metric.ATTRIBUTE},
			"query_text":           {metricObject.QueryText, metric.ATTRIBUTE},
			"database_name":        {metricObject.DatabaseName, metric.ATTRIBUTE},
			"schema_name":          {metricObject.SchemaName, metric.ATTRIBUTE},
			"execution_count":      {metricObject.ExecutionCount, metric.GAUGE},
			"avg_cpu_time_ms":      {metricObject.AvgCPUTimeMs, metric.GAUGE},
			"avg_elapsed_time_ms":  {metricObject.AvgElapsedTimeMs, metric.GAUGE},
			"avg_disk_reads":       {metricObject.AvgDiskReads, metric.GAUGE},
			"avg_disk_writes":      {metricObject.AvgDiskWrites, metric.GAUGE},
			"has_full_table_scan":  {metricObject.HasFullTableScan, metric.ATTRIBUTE},
			"statement_type":       {metricObject.StatementType, metric.ATTRIBUTE},
			"collection_timestamp": {metricObject.CollectionTimestamp, metric.ATTRIBUTE},
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
