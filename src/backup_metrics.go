package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

// backupMetrics defines metrics for detecting active backup operations
var backupMetrics = map[string][]interface{}{
	"db.backupActive.total":     {"total_backup_active", metric.GAUGE},
	"db.backupActive.logical":   {"logical_backup_active", metric.GAUGE},
	"db.backupActive.physical":  {"physical_backup_active", metric.GAUGE},
	"db.backupActive.tableLock": {"table_lock_active", metric.GAUGE},
	"db.backupActive.other":     {"other_backup_active", metric.GAUGE},
}

// backupHistoryMetrics defines metrics for historical backup operations from performance_schema
var backupHistoryMetrics = map[string][]interface{}{
	"db.backupHistory.total":                {"total_backup_history", metric.GAUGE},
	"db.backupHistory.logical.count":        {"logical_backup_count", metric.GAUGE},
	"db.backupHistory.logical.avgDuration":  {"logical_backup_avg_duration", metric.GAUGE},
	"db.backupHistory.logical.maxDuration":  {"logical_backup_max_duration", metric.GAUGE},
	"db.backupHistory.physical.count":       {"physical_backup_count", metric.GAUGE},
	"db.backupHistory.physical.avgDuration": {"physical_backup_avg_duration", metric.GAUGE},
	"db.backupHistory.physical.maxDuration": {"physical_backup_max_duration", metric.GAUGE},
	"db.backupHistory.tableLock.count":      {"table_lock_count", metric.GAUGE},
	"db.backupHistory.tableLock.avgDuration": {"table_lock_avg_duration", metric.GAUGE},
	"db.backupHistory.tableLock.maxDuration": {"table_lock_max_duration", metric.GAUGE},
	"db.backupHistory.other.count":          {"other_backup_count", metric.GAUGE},
	"db.backupHistory.other.avgDuration":    {"other_backup_avg_duration", metric.GAUGE},
	"db.backupHistory.other.maxDuration":    {"other_backup_max_duration", metric.GAUGE},
}

// backupMetricsQuery detects active backup operations using information_schema.processlist
// This captures running commands that indicate backup operations
// Backup types:
// - Logical Backup: mysqldump using START TRANSACTION WITH CONSISTENT SNAPSHOT
// - Physical Backup: Physical backups using FLUSH TABLES WITH READ LOCK
// - Table Lock: Table-level locks using LOCK TABLES
// - Other: Other backup-related activities
const backupMetricsQuery = `
SELECT
    COUNT(DISTINCT id) AS total_backup_active,
    SUM(CASE WHEN info LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN 1 ELSE 0 END) AS logical_backup_active,
    SUM(CASE WHEN info LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN 1 ELSE 0 END) AS physical_backup_active,
    SUM(CASE WHEN info LIKE '%LOCK TABLES%' THEN 1 ELSE 0 END) AS table_lock_active,
    SUM(CASE
        WHEN info NOT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%'
        AND info NOT LIKE '%FLUSH TABLES%WITH READ LOCK%'
        AND info NOT LIKE '%LOCK TABLES%'
        AND (info LIKE '%FLUSH%' OR info LIKE '%LOCK%')
        THEN 1 ELSE 0
    END) AS other_backup_active
FROM information_schema.processlist
WHERE id != CONNECTION_ID()
AND info IS NOT NULL
AND (info LIKE '%LOCK TABLES%'
     OR info LIKE '%FLUSH%'
     OR info LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%')
`

// backupHistoryMetricsQuery analyzes historical backup operations from performance_schema
// Returns aggregated statistics for each backup type including counts and duration metrics
// Backup types:
// - Logical Backup: mysqldump using START TRANSACTION WITH CONSISTENT SNAPSHOT
// - Physical Backup: Physical backups using FLUSH TABLES WITH READ LOCK
// - Table Lock: Table-level locks using LOCK TABLES
// - Other: Other backup-related activities
const backupHistoryMetricsQuery = `
SELECT
    COUNT(*) AS total_backup_history,
    SUM(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN 1 ELSE 0 END) AS logical_backup_count,
    AVG(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS logical_backup_avg_duration,
    MAX(CASE WHEN SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS logical_backup_max_duration,
    SUM(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN 1 ELSE 0 END) AS physical_backup_count,
    AVG(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS physical_backup_avg_duration,
    MAX(CASE WHEN SQL_TEXT LIKE '%FLUSH TABLES%WITH READ LOCK%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS physical_backup_max_duration,
    SUM(CASE WHEN SQL_TEXT LIKE '%LOCK TABLES%' THEN 1 ELSE 0 END) AS table_lock_count,
    AVG(CASE WHEN SQL_TEXT LIKE '%LOCK TABLES%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS table_lock_avg_duration,
    MAX(CASE WHEN SQL_TEXT LIKE '%LOCK TABLES%' THEN TIMER_WAIT/1000000000000 ELSE NULL END) AS table_lock_max_duration,
    SUM(CASE
        WHEN SQL_TEXT NOT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%'
        AND SQL_TEXT NOT LIKE '%FLUSH TABLES%WITH READ LOCK%'
        AND SQL_TEXT NOT LIKE '%LOCK TABLES%' THEN 1
        ELSE 0
    END) AS other_backup_count,
    AVG(CASE
        WHEN SQL_TEXT NOT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%'
        AND SQL_TEXT NOT LIKE '%FLUSH TABLES%WITH READ LOCK%'
        AND SQL_TEXT NOT LIKE '%LOCK TABLES%' THEN TIMER_WAIT/1000000000000
        ELSE NULL
    END) AS other_backup_avg_duration,
    MAX(CASE
        WHEN SQL_TEXT NOT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%'
        AND SQL_TEXT NOT LIKE '%FLUSH TABLES%WITH READ LOCK%'
        AND SQL_TEXT NOT LIKE '%LOCK TABLES%' THEN TIMER_WAIT/1000000000000
        ELSE NULL
    END) AS other_backup_max_duration
FROM performance_schema.events_statements_history
WHERE (SQL_TEXT LIKE '%LOCK TABLES%'
    OR SQL_TEXT LIKE '%FLUSH%'
    OR SQL_TEXT LIKE '%START TRANSACTION WITH CONSISTENT SNAPSHOT%')
`
