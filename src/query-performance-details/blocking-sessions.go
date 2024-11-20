package query_performance_details

import (
	"context"
	"database/sql"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

type BlockingSessionMetrics struct {
	BlockedTxnID     sql.NullString `json:"blocked_txn_id" db:"blocked_txn_id"`
	BlockedThreadID  sql.NullInt64  `json:"blocked_thread_id" db:"blocked_thread_id"`
	BlockedUser      sql.NullString `json:"blocked_user" db:"blocked_user"`
	BlockedHost      sql.NullString `json:"blocked_host" db:"blocked_host"`
	BlockedDB        sql.NullString `json:"blocked_db" db:"blocked_db"`
	BlockingTxnID    sql.NullString `json:"blocking_txn_id" db:"blocking_txn_id"`
	BlockingThreadID sql.NullInt64  `json:"blocking_thread_id" db:"blocking_thread_id"`
	BlockingUser     sql.NullString `json:"blocking_user" db:"blocking_user"`
	BlockingHost     sql.NullString `json:"blocking_host" db:"blocking_host"`
	BlockingDB       sql.NullString `json:"blocking_db" db:"blocking_db"`
	BlockedQuery     sql.NullString `json:"blocked_query" db:"blocked_query"`
	BlockingQuery    sql.NullString `json:"blocking_query" db:"blocking_query"`
}

// Commenting out the unused function
func collectBlockingSessionMetrics(db dataSource) ([]BlockingSessionMetrics, error) {
	metrics, err := fetchBlockingSessionMetrics(db)
	if err != nil {
		log.Error("Failed to collect blocking session metrics: %v", err)
		return nil, err
	}
	return metrics, nil
}

func fetchBlockingSessionMetrics(db dataSource) ([]BlockingSessionMetrics, error) {
	query := `
		SELECT 
                      r.trx_id AS blocked_txn_id,
                      r.trx_mysql_thread_id AS blocked_thread_id,
                      wt.PROCESSLIST_USER AS blocked_user,
                      wt.PROCESSLIST_HOST AS blocked_host,
                      wt.PROCESSLIST_DB AS blocked_db,
                      b.trx_id AS blocking_txn_id,
                      b.trx_mysql_thread_id AS blocking_thread_id,
                      bt.PROCESSLIST_USER AS blocking_user,
                      bt.PROCESSLIST_HOST AS blocking_host,
                      bt.PROCESSLIST_DB AS blocking_db,
                      es_waiting.DIGEST_TEXT AS blocked_query,
                      es_blocking.DIGEST_TEXT AS blocking_query
                  FROM 
                      performance_schema.data_lock_waits w
                  JOIN 
                      performance_schema.threads wt ON wt.THREAD_ID = w.REQUESTING_THREAD_ID
                  JOIN 
                      information_schema.innodb_trx r ON r.trx_mysql_thread_id = wt.PROCESSLIST_ID
                  JOIN 
                      performance_schema.threads bt ON bt.THREAD_ID = w.BLOCKING_THREAD_ID
                  JOIN 
                      information_schema.innodb_trx b ON b.trx_mysql_thread_id = bt.PROCESSLIST_ID
                  JOIN 
                      performance_schema.events_statements_current esc_waiting ON esc_waiting.THREAD_ID = wt.THREAD_ID
                  JOIN 
                      performance_schema.events_statements_summary_by_digest es_waiting 
                      ON esc_waiting.DIGEST = es_waiting.DIGEST
                  JOIN 
                      performance_schema.events_statements_current esc_blocking ON esc_blocking.THREAD_ID = bt.THREAD_ID
                  JOIN 
                      performance_schema.events_statements_summary_by_digest es_blocking 
                      ON esc_blocking.DIGEST = es_blocking.DIGEST;
	`
	rows, err := db.QueryxContext(context.Background(), query)
	if err != nil {
		log.Error("Failed to execute blocking session query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var metrics []BlockingSessionMetrics
	for rows.Next() {
		var metric BlockingSessionMetrics
		if err := rows.StructScan(&metric); err != nil {
			log.Error("Failed to scan blocking session metrics: %v", err)
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		log.Error("Error iterating over blocking session metrics rows: %v", err)
		return nil, err
	}

	return metrics, nil
}

// CreateBlockingSessionMetrics creates a new set of metrics for the given BlockingSessionMetrics slice
func populateBlockingSessionMetrics(e *integration.Entity, args arguments.ArgumentList, metrics []BlockingSessionMetrics) error {
	for _, metricData := range metrics {
		// Create a new metric set for each row
		ms := createMetricSet(e, "MysqlBlockingSessionSample", args)
		metricsMap := map[string]struct {
			Value      interface{}
			MetricType metric.SourceType
		}{
			"blocked_txn_id":     {getStringValue(metricData.BlockedTxnID), metric.ATTRIBUTE},
			"blocked_thread_id":  {getInt64Value(metricData.BlockedThreadID), metric.GAUGE},
			"blocked_user":       {getStringValue(metricData.BlockedUser), metric.ATTRIBUTE},
			"blocked_host":       {getStringValue(metricData.BlockedHost), metric.ATTRIBUTE},
			"blocked_db":         {getStringValue(metricData.BlockedDB), metric.ATTRIBUTE},
			"blocking_txn_id":    {getStringValue(metricData.BlockingTxnID), metric.ATTRIBUTE},
			"blocking_thread_id": {getInt64Value(metricData.BlockingThreadID), metric.GAUGE},
			"blocking_user":      {getStringValue(metricData.BlockingUser), metric.ATTRIBUTE},
			"blocking_host":      {getStringValue(metricData.BlockingHost), metric.ATTRIBUTE},
			"blocking_db":        {getStringValue(metricData.BlockingDB), metric.ATTRIBUTE},
			"blocked_query":      {getStringValue(metricData.BlockedQuery), metric.ATTRIBUTE},
			"blocking_query":     {getStringValue(metricData.BlockingQuery), metric.ATTRIBUTE},
		}

		for metricName, data := range metricsMap {
			err := ms.SetMetric(metricName, data.Value, data.MetricType)
			if err != nil {
				log.Warn("Error setting value:  %s", err)
				continue
			}
		}
	}

	return nil
}

func getInt64Value(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}
