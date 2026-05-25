package main

import (
	"database/sql"
	"sync"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

// mdlWarningOnce ensures the MDL instrument warning is logged at most once per process run.
var mdlWarningOnce sync.Once

var backupMetrics = map[string][]interface{}{
	"db.backupActive.total":     {"total_backup_active", metric.GAUGE},
	"db.backupActive.logical":   {"logical_backup_active", metric.GAUGE},
	"db.backupActive.physical":  {"physical_backup_active", metric.GAUGE},
	"db.backupActive.tableLock": {"table_lock_active", metric.GAUGE},
}

var backupHistoryMetrics = map[string][]interface{}{
	"db.backupHistory.total":                 {"total_backup_history", metric.GAUGE},
	"db.backupHistory.logical.count":         {"logical_backup_count", metric.GAUGE},
	"db.backupHistory.logical.avgDuration":   {"logical_backup_avg_duration", metric.GAUGE},
	"db.backupHistory.logical.maxDuration":   {"logical_backup_max_duration", metric.GAUGE},
	"db.backupHistory.physical.count":        {"physical_backup_count", metric.GAUGE},
	"db.backupHistory.physical.avgDuration":  {"physical_backup_avg_duration", metric.GAUGE},
	"db.backupHistory.physical.maxDuration":  {"physical_backup_max_duration", metric.GAUGE},
	"db.backupHistory.tableLock.count":       {"table_lock_count", metric.GAUGE},
	"db.backupHistory.tableLock.avgDuration": {"table_lock_avg_duration", metric.GAUGE},
	"db.backupHistory.tableLock.maxDuration": {"table_lock_max_duration", metric.GAUGE},
}

// backupMetricsQuery detects active backup operations via performance_schema.metadata_locks
// and information_schema.innodb_trx. Requires MDL instrument enabled on MariaDB.
const backupMetricsQuery = `
SELECT
    COALESCE(b.physical_backup_active, 0) + COALESCE(b.table_lock_active, 0) + COALESCE(l.logical_backup_active, 0) AS total_backup_active,
    COALESCE(l.logical_backup_active, 0) AS logical_backup_active,
    COALESCE(b.physical_backup_active, 0) AS physical_backup_active,
    COALESCE(b.table_lock_active, 0) AS table_lock_active
FROM (
    SELECT
        -- Physical backups: OBJECT_TYPE='BACKUP' with LOCK_TYPE LIKE 'BACKUP_FTWRL%' (MariaDB)
        COUNT(DISTINCT CASE
            WHEN OBJECT_TYPE = 'BACKUP'
                AND LOCK_TYPE LIKE 'BACKUP_FTWRL%'
                AND LOCK_STATUS = 'GRANTED'
            THEN OWNER_THREAD_ID
        END) AS physical_backup_active,
        -- Table locks: LOCK_DURATION IN ('EXPLICIT','TRANSACTION') covers MySQL and MariaDB respectively
        COUNT(DISTINCT CASE
            WHEN OBJECT_TYPE = 'TABLE'
                AND LOCK_TYPE IN ('SHARED_READ', 'SHARED_NO_READ_WRITE', 'EXCLUSIVE')
                AND LOCK_STATUS = 'GRANTED'
                AND LOCK_DURATION IN ('EXPLICIT', 'TRANSACTION')
                AND OBJECT_SCHEMA NOT IN ('performance_schema', 'information_schema', 'sys')
            THEN OWNER_THREAD_ID
        END) AS table_lock_active
    FROM performance_schema.metadata_locks
    WHERE OWNER_THREAD_ID IS NOT NULL
) b,
(
    SELECT
        -- Logical backups: read-only transactions running >= 30s with no row modifications
        COUNT(DISTINCT trx_id) AS logical_backup_active
    FROM information_schema.innodb_trx
    WHERE trx_state = 'RUNNING'
        AND trx_rows_modified = 0
        AND TIMESTAMPDIFF(SECOND, trx_started, NOW()) >= 30
) l
`

// backupHistoryMetricsQuery aggregates backup history from performance_schema.events_statements_history.
// Ring buffer holds ~10 events per thread; older events may be evicted on busy servers.
const backupHistoryMetricsQuery = `
SELECT
    COALESCE(SUM(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN 1 ELSE 0 END), 0)
    + COALESCE(SUM(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN 1 ELSE 0 END), 0)
    + COALESCE(SUM(CASE WHEN SQL_TEXT LIKE 'LOCK TABLES %' THEN 1 ELSE 0 END), 0) AS total_backup_history,
    COALESCE(SUM(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN 1 ELSE 0 END), 0) AS logical_backup_count,
    COALESCE(AVG(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS logical_backup_avg_duration,
    COALESCE(MAX(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS logical_backup_max_duration,
    COALESCE(SUM(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN 1 ELSE 0 END), 0) AS physical_backup_count,
    COALESCE(AVG(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS physical_backup_avg_duration,
    COALESCE(MAX(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS physical_backup_max_duration,
    COALESCE(SUM(CASE WHEN SQL_TEXT LIKE 'LOCK TABLES %' THEN 1 ELSE 0 END), 0) AS table_lock_count,
    COALESCE(AVG(CASE WHEN SQL_TEXT LIKE 'LOCK TABLES %' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS table_lock_avg_duration,
    COALESCE(MAX(CASE WHEN SQL_TEXT LIKE 'LOCK TABLES %' THEN TIMER_WAIT/1000000000000 ELSE NULL END), 0) AS table_lock_max_duration
FROM performance_schema.events_statements_history
WHERE (SQL_TEXT LIKE 'LOCK TABLES %'
    OR SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%'
    OR SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%')
`

// backupMetricsQueryFallback is used for MySQL 5.7 / MariaDB < 10.5 without metadata_locks.
// Less accurate: detects only statements currently executing, not held locks.
const backupMetricsQueryFallback = `
SELECT
    COALESCE(p.table_lock_active, 0) + COALESCE(t.logical_backup_active, 0) AS total_backup_active,
    COALESCE(t.logical_backup_active, 0) AS logical_backup_active,
    0 AS physical_backup_active,
    COALESCE(p.table_lock_active, 0) AS table_lock_active
FROM (
    SELECT
        -- 'LOCK TABLES %' (starts-with) avoids matching UNLOCK TABLES statements
        COUNT(DISTINCT CASE
            WHEN INFO LIKE 'LOCK TABLES %'
            THEN ID
        END) AS table_lock_active
    FROM information_schema.processlist
) p,
(
    SELECT
        COUNT(DISTINCT trx_id) AS logical_backup_active
    FROM information_schema.innodb_trx
    WHERE trx_state = 'RUNNING'
        AND trx_rows_modified = 0
        AND TIMESTAMPDIFF(SECOND, trx_started, NOW()) >= 30
) t
`

// queryCount executes a SELECT COUNT(*) query and returns the result.
func queryCount(db *sql.DB, query string) (int, error) {
	var count int
	err := db.QueryRow(query).Scan(&count)
	return count, err
}

// supportsMetadataLocks returns true if performance_schema.metadata_locks exists (MySQL 8.0+ / MariaDB 10.5+).
func supportsMetadataLocks(db *sql.DB) bool {
	count, err := queryCount(db, `
		SELECT COUNT(*)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = 'performance_schema'
		  AND TABLE_NAME = 'metadata_locks'
	`)
	if err != nil {
		log.Warn("Failed to check for metadata_locks table: %v", err)
		return false
	}
	return count > 0
}

// warnIfMDLInstrumentDisabled warns once if the MDL instrument is disabled (MariaDB default).
// To fix: add performance-schema-instrument='wait/lock/metadata/sql/mdl=ON' to my.cnf.
func warnIfMDLInstrumentDisabled(db *sql.DB) {
	enabled, err := queryCount(db, `
		SELECT COUNT(*)
		FROM performance_schema.setup_instruments
		WHERE NAME = 'wait/lock/metadata/sql/mdl'
		  AND ENABLED = 'YES'
	`)
	if err != nil {
		log.Debug("Could not check MDL instrument status: %v", err)
		return
	}
	if enabled > 0 {
		return
	}
	mdlWarningOnce.Do(func() {
		log.Warn("MDL instrument 'wait/lock/metadata/sql/mdl' is disabled in performance_schema. " +
			"Active backup metrics (db.backupActive.*) will always report 0. " +
			"To enable accurate detection, add to your database configuration: " +
			"performance-schema-instrument='wait/lock/metadata/sql/mdl=ON'")
	})
}

// getBackupMetricsQuery returns the metadata_locks query if supported, otherwise the processlist fallback.
func getBackupMetricsQuery(db *sql.DB) string {
	if supportsMetadataLocks(db) {
		warnIfMDLInstrumentDisabled(db)
		log.Debug("Using metadata_locks-based backup detection (MySQL 8.0+/MariaDB 10.5+)")
		return backupMetricsQuery
	}

	log.Info("metadata_locks table not available, using fallback query (MySQL 5.7/MariaDB < 10.5)")
	return backupMetricsQueryFallback
}
