package main

import (
	"database/sql"
	"sync"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

// mdlWarningOnce ensures the MDL instrument warning is logged at most once per process run.
var mdlWarningOnce sync.Once

// backupMetrics defines metrics for detecting active backup operations
var backupMetrics = map[string][]interface{}{
	"db.backupActive.total":     {"total_backup_active", metric.GAUGE},
	"db.backupActive.logical":   {"logical_backup_active", metric.GAUGE},
	"db.backupActive.physical":  {"physical_backup_active", metric.GAUGE},
	"db.backupActive.tableLock": {"table_lock_active", metric.GAUGE},
}

// backupHistoryMetrics defines metrics for historical backup operations from performance_schema
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

// backupMetricsQuery detects active backup operations using performance_schema.metadata_locks
// and information_schema.innodb_trx
// This is more reliable than checking processlist.info which only shows the currently executing statement
//
// MariaDB-specific implementation using metadata_locks table with OWNER_THREAD_ID
//
// Detection methods:
//
//   - Physical Backup: FLUSH TABLES WITH READ LOCK creates OBJECT_TYPE='BACKUP' with LOCK_TYPE='BACKUP_FTWRL1'
//
//   - Table Lock: LOCK TABLES creates SHARED_READ/SHARED_WRITE locks on user tables
//     Note: MariaDB uses LOCK_DURATION='TRANSACTION' for LOCK TABLES (not 'EXPLICIT')
//     We detect sessions holding explicit table locks as indicator of LOCK TABLES usage
//
//   - Logical Backup: Long-running read-only transactions (e.g., START TRANSACTION WITH CONSISTENT SNAPSHOT)
//     Detected via information_schema.innodb_trx looking for transactions that:
//
//   - Are in RUNNING state
//
//   - Have not modified any rows (trx_rows_modified = 0)
//
//   - Have been running for at least 30 seconds (reduces false positives from long SELECT queries)
//
//     NOTE: This heuristic may still catch some false positives (long reports, analytics queries).
//     To reduce false positives further, you can:
//
//   - Increase the threshold (30s default is conservative for most backup tools)
//
//   - Exclude specific users by filtering on trx_mysql_thread_id
//
const backupMetricsQuery = `
SELECT
    COALESCE(b.physical_backup_active, 0) + COALESCE(b.table_lock_active, 0) + COALESCE(l.logical_backup_active, 0) AS total_backup_active,
    COALESCE(l.logical_backup_active, 0) AS logical_backup_active,
    COALESCE(b.physical_backup_active, 0) AS physical_backup_active,
    COALESCE(b.table_lock_active, 0) AS table_lock_active
FROM (
    SELECT
        -- Physical backups: FLUSH TABLES WITH READ LOCK (MariaBackup, Percona XtraBackup)
        -- Creates OBJECT_TYPE='BACKUP' with LOCK_TYPE='BACKUP_FTWRL1'
        COUNT(DISTINCT CASE
            WHEN OBJECT_TYPE = 'BACKUP'
                AND LOCK_TYPE LIKE 'BACKUP_FTWRL%'
                AND LOCK_STATUS = 'GRANTED'
            THEN OWNER_THREAD_ID
        END) AS physical_backup_active,
        -- Table locks: LOCK TABLES ... READ/WRITE (mysqldump --lock-tables, write-lock backups)
        -- SHARED_READ: read locks (LOCK TABLES t READ)
        -- SHARED_NO_READ_WRITE/EXCLUSIVE: write locks (LOCK TABLES t WRITE)
        -- LOCK_DURATION IN ('EXPLICIT', 'TRANSACTION') covers both engines:
        --   MySQL:   LOCK TABLES creates EXPLICIT duration locks
        --   MariaDB: LOCK TABLES creates TRANSACTION duration locks
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
        -- Logical backups: Long-running read-only transactions
        -- Typical pattern: START TRANSACTION WITH CONSISTENT SNAPSHOT followed by data reads
        -- Using 30-second threshold to reduce false positives from regular queries
        COUNT(DISTINCT trx_id) AS logical_backup_active
    FROM information_schema.innodb_trx
    WHERE trx_state = 'RUNNING'
        AND trx_rows_modified = 0
        AND TIMESTAMPDIFF(SECOND, trx_started, NOW()) >= 30
) l
`

// backupHistoryMetricsQuery analyzes historical backup operations from performance_schema
// Returns aggregated statistics for each backup type including counts and duration metrics
// Backup types:
// - Logical Backup: mysqldump using START TRANSACTION WITH CONSISTENT SNAPSHOT
// - Physical Backup: Physical backups using FLUSH TABLES WITH READ LOCK
// - Table Lock: Table-level locks using LOCK TABLES
//
// Note: events_statements_history is a ring buffer with limited size (typically 10 events per thread)
// Older events may have been evicted, so this represents recent history only
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

// backupMetricsQueryFallback is for older versions (MySQL 5.7, MariaDB < 10.5)
// that don't have performance_schema.metadata_locks table
// This uses information_schema.processlist which is less accurate but universally available
//
// Limitations:
// - Can only detect backups currently executing LOCK/FLUSH statements
// - Cannot detect held locks (once statement completes, lock disappears from processlist)
// - Less reliable than metadata_locks approach
const backupMetricsQueryFallback = `
SELECT
    COALESCE(p.table_lock_active, 0) + COALESCE(t.logical_backup_active, 0) AS total_backup_active,
    COALESCE(t.logical_backup_active, 0) AS logical_backup_active,
    0 AS physical_backup_active,
    COALESCE(p.table_lock_active, 0) AS table_lock_active
FROM (
    SELECT
        -- Table locks: Sessions executing LOCK TABLES statements
        -- Using 'LOCK TABLES %' (starts-with) to avoid matching UNLOCK TABLES statements
        COUNT(DISTINCT CASE
            WHEN INFO LIKE 'LOCK TABLES %'
            THEN ID
        END) AS table_lock_active
    FROM information_schema.processlist
) p,
(
    SELECT
        -- Logical backups: Long-running read-only transactions (InnoDB only)
        -- Uses same heuristic as main query (30+ seconds, no modifications)
        COUNT(DISTINCT trx_id) AS logical_backup_active
    FROM information_schema.innodb_trx
    WHERE trx_state = 'RUNNING'
        AND trx_rows_modified = 0
        AND TIMESTAMPDIFF(SECOND, trx_started, NOW()) >= 30
) t
`

// supportsMetadataLocks checks if the database supports performance_schema.metadata_locks
// Returns true for MySQL 8.0+ and MariaDB 10.5+
func supportsMetadataLocks(db *sql.DB) bool {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = 'performance_schema'
		  AND TABLE_NAME = 'metadata_locks'
	`).Scan(&count)

	if err != nil {
		log.Warn("Failed to check for metadata_locks table: %v", err)
		return false
	}

	return count > 0
}

// warnIfMDLInstrumentDisabled logs a warning if the MDL instrument is disabled in performance_schema.
// On MariaDB the instrument is off by default, which leaves metadata_locks empty and causes
// active backup metrics to report 0. Customers must enable it in their database configuration:
//
//	performance-schema-instrument='wait/lock/metadata/sql/mdl=ON'
func warnIfMDLInstrumentDisabled(db *sql.DB) {
	var enabled int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM performance_schema.setup_instruments
		WHERE NAME = 'wait/lock/metadata/sql/mdl'
		  AND ENABLED = 'YES'
	`).Scan(&enabled)
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

// getBackupMetricsQuery returns the appropriate backup metrics query based on database version
func getBackupMetricsQuery(db *sql.DB) string {
	if supportsMetadataLocks(db) {
		warnIfMDLInstrumentDisabled(db)
		log.Debug("Using metadata_locks-based backup detection (MySQL 8.0+/MariaDB 10.5+)")
		return backupMetricsQuery
	}

	log.Info("metadata_locks table not available, using fallback query (MySQL 5.7/MariaDB < 10.5)")
	return backupMetricsQueryFallback
}
