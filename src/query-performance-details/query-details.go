package query_performance_details

import (
	"context"
	"database/sql"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

type QueryMetrics struct {
	QueryID             string         `db:"query_id" json:"query.id"`
	QueryText           sql.NullString `db:"query_text" json:"query.text"`
	DatabaseName        sql.NullString `db:"database_name" json:"database.name"`
	SchemaName          string         `db:"schema_name" json:"schema.name"`
	ExecutionCount      uint64         `db:"execution_count" json:"execution.count"`
	AvgCPUTimeMs        float64        `db:"avg_cpu_time_ms" json:"avg.cpuTime.ms"`
	AvgElapsedTimeMs    float64        `db:"avg_elapsed_time_ms" json:"avg.elapsedTime.ms"`
	AvgDiskReads        float64        `db:"avg_disk_reads" json:"avg.diskReads"`
	AvgDiskWrites       float64        `db:"avg_disk_writes" json:"avg.diskWrites"`
	HasFullTableScan    string         `db:"has_full_table_scan" json:"has.fullTableScan"`
	StatementType       string         `db:"statement_type" json:"statement.type"`
	CollectionTimestamp string         `db:"collection_timestamp" json:"collection.timestamp"`
	CollectedAt         time.Time      `db:"-" json:"collected.at"`
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
        WHERE LAST_SEEN >= UTC_TIMESTAMP() - INTERVAL 10 SECOND
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
