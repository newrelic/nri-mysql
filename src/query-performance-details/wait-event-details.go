package query_performance_details

import (
	"context"
	"database/sql"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

type WaitEventQueryMetrics struct {
	TotalWaitTimeMs     float64        `json:"total_wait_time_ms" db:"total_wait_time_ms"`
	QueryID             sql.NullString `json:"query_id" db:"query_id"`
	QueryText           sql.NullString `json:"query_text" db:"query_text"`
	DatabaseName        sql.NullString `json:"database_name" db:"database_name"`
	WaitCategory        string         `json:"wait_category" db:"wait_category"`
	CollectionTimestamp string         `json:"collection_timestamp" db:"collection_timestamp"`
	InstanceID          string         `json:"instance_id" db:"instance_id"`
	WaitEventName       string         `json:"wait_event_name" db:"wait_event_name"`
	WaitingTasksCount   uint64         `json:"waiting_tasks_count" db:"waiting_tasks_count"`
}

// Commenting out the unused function
func collectWaitEventQueryMetrics(db dataSource) ([]WaitEventQueryMetrics, error) {
	metrics, err := collectWaitEventMetrics(db)
	if err != nil {
		log.Error("Failed to collect wait event metrics: %v", err)
		return nil, err
	}
	return metrics, nil
}

func collectWaitEventMetrics(db dataSource) ([]WaitEventQueryMetrics, error) {
	query := `
		SELECT
            LEFT(UPPER(SHA2(eshl.SQL_TEXT, 256)), 16) AS query_id,
            ewhl.OBJECT_INSTANCE_BEGIN AS instance_id,
            eshl.CURRENT_SCHEMA AS database_name,
            ewhl.EVENT_NAME AS wait_event_name,
            CASE
                WHEN ewhl.EVENT_NAME LIKE 'wait/io/file/innodb/%' THEN 'InnoDB File IO'
                WHEN ewhl.EVENT_NAME LIKE 'wait/io/file/sql/%' THEN 'SQL File IO'
                WHEN ewhl.EVENT_NAME LIKE 'wait/io/socket/%' THEN 'Network IO'
                WHEN ewhl.EVENT_NAME LIKE 'wait/synch/cond/%' THEN 'Condition Wait'
                WHEN ewhl.EVENT_NAME LIKE 'wait/synch/mutex/%' THEN 'Mutex'
                WHEN ewhl.EVENT_NAME LIKE 'wait/lock/table/%' THEN 'Table Lock'
                WHEN ewhl.EVENT_NAME LIKE 'wait/lock/metadata/%' THEN 'Metadata Lock'
                WHEN ewhl.EVENT_NAME LIKE 'wait/lock/transaction/%' THEN 'Transaction Lock'
                ELSE 'Other'
            END AS wait_category,
                ROUND(SUM(ewhl.TIMER_WAIT) / 1000000000000, 3) AS total_wait_time_ms,
                SUM(ewsg.COUNT_STAR) AS waiting_tasks_count,
                eshl.SQL_TEXT AS query_text,
                DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
            FROM performance_schema.events_waits_history_long ewhl
            JOIN performance_schema.events_statements_history_long eshl 
            ON
                ewhl.THREAD_ID = eshl.THREAD_ID
            JOIN
                 performance_schema.events_waits_summary_global_by_event_name ewsg 
            ON
                ewhl.EVENT_NAME = ewsg.EVENT_NAME
            GROUP BY
                query_id,
                instance_id,
                wait_event_name,
                wait_category,
                database_name,
                eshl.SQL_TEXT
            ORDER BY total_wait_time_ms DESC
            LIMIT 10;
	`

	rows, err := db.QueryxContext(context.Background(), query)
	if err != nil {
		log.Error("Failed to execute query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var metrics []WaitEventQueryMetrics
	for rows.Next() {
		var metric WaitEventQueryMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan row: %v", err)
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

// populateWaitEventMetrics populates the metric set with the wait event metrics.
func populateWaitEventMetrics(e *integration.Entity, args arguments.ArgumentList, metrics []WaitEventQueryMetrics) error {
	for _, metricData := range metrics {
		// Create a new metric set for each row
		ms := createMetricSet(e, "MysqlWaitEventSample", args)
		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{

			"total_wait_time_ms":   {metricData.TotalWaitTimeMs, metric.GAUGE},
			"query_id":             {getStringValue(metricData.QueryID), metric.ATTRIBUTE},
			"query_text":           {getStringValue(metricData.QueryText), metric.ATTRIBUTE},
			"database_name":        {getStringValue(metricData.DatabaseName), metric.ATTRIBUTE},
			"wait_category":        {metricData.WaitCategory, metric.ATTRIBUTE},
			"collection_timestamp": {metricData.CollectionTimestamp, metric.ATTRIBUTE},
			"instance_id":          {metricData.InstanceID, metric.ATTRIBUTE},
			"wait_event_name":      {metricData.WaitEventName, metric.ATTRIBUTE},
			"waiting_tasks_count":  {int(metricData.WaitingTasksCount), metric.GAUGE},
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
