package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var defaultMetricsBase = map[string][]interface{}{
	"net.abortedClientsPerSecond":                 {"Aborted_clients", metric.PRATE},
	"net.abortedConnectsPerSecond":                {"Aborted_connects", metric.PRATE},
	"net.bytesReceivedPerSecond":                  {"Bytes_received", metric.PRATE},
	"net.bytesSentPerSecond":                      {"Bytes_sent", metric.PRATE},
	"net.connectionErrorsMaxConnectionsPerSecond": {"Connection_errors_max_connections", metric.PRATE},
	"net.connectionsPerSecond":                    {"Connections", metric.PRATE},
	"net.maxUsedConnections":                      {"Max_used_connections", metric.GAUGE},
	"net.threadsConnected":                        {"Threads_connected", metric.GAUGE},
	"net.threadsRunning":                          {"Threads_running", metric.GAUGE},
	"query.comCommitPerSecond":                    {"Com_commit", metric.PRATE},
	"query.comDeletePerSecond":                    {"Com_delete", metric.PRATE},
	"query.comDeleteMultiPerSecond":               {"Com_delete_multi", metric.PRATE},
	"query.comInsertPerSecond":                    {"Com_insert", metric.PRATE},
	"query.comInsertSelectPerSecond":              {"Com_insert_select", metric.PRATE},
	"query.comReplaceSelectPerSecond":             {"Com_replace_select", metric.PRATE},
	"query.comRollbackPerSecond":                  {"Com_rollback", metric.PRATE},
	"query.comSelectPerSecond":                    {"Com_select", metric.PRATE},
	"query.comUpdatePerSecond":                    {"Com_update", metric.PRATE},
	"query.comUpdateMultiPerSecond":               {"Com_update_multi", metric.PRATE},
	"db.handlerRollbackPerSecond":                 {"Handler_rollback", metric.PRATE},
	"query.preparedStmtCountPerSecond":            {"Prepared_stmt_count", metric.PRATE},
	"query.queriesPerSecond":                      {"Queries", metric.PRATE},
	"query.questionsPerSecond":                    {"Questions", metric.PRATE},
	"query.slowQueriesPerSecond":                  {"Slow_queries", metric.PRATE},
	"db.innodb.bufferPoolPagesData":               {"Innodb_buffer_pool_pages_data", metric.GAUGE},
	"db.innodb.bufferPoolPagesFree":               {"Innodb_buffer_pool_pages_free", metric.GAUGE},
	"db.innodb.bufferPoolPagesTotal":              {"Innodb_buffer_pool_pages_total", metric.GAUGE},
	"db.innodb.dataReadBytesPerSecond":            {"Innodb_data_read", metric.PRATE},
	"db.innodb.dataWrittenBytesPerSecond":         {"Innodb_data_written", metric.PRATE},
	"db.innodb.logWaitsPerSecond":                 {"Innodb_log_waits", metric.PRATE},
	"db.innodb.rowLockCurrentWaits":               {"Innodb_row_lock_current_waits", metric.GAUGE},
	"db.innodb.rowLockTimeAvg":                    {"Innodb_row_lock_time_avg", metric.GAUGE},
	"db.innodb.rowLockWaitsPerSecond":             {"Innodb_row_lock_waits", metric.PRATE},
	"db.openFiles":                                {"Open_files", metric.GAUGE},
	"db.openTables":                               {"Open_tables", metric.GAUGE},
	"db.openedTablesPerSecond":                    {"Opened_tables", metric.PRATE},
	"db.tablesLocksWaitedPerSecond":               {"Table_locks_waited", metric.PRATE},
	"software.edition":                            {"version_comment", metric.ATTRIBUTE},
	"software.version":                            {"version", metric.ATTRIBUTE},
	"cluster.nodeType":                            {"node_type", metric.ATTRIBUTE},
	// If a cluster instance is not a slave, then the metric cluster.slaveRunning will be removed.
	"cluster.slaveRunning": {slaveRunningAsNumber, metric.GAUGE},
}

var defaultMetricsBelowVersion8 = map[string][]interface{}{
	"db.qCacheFreeMemoryBytes":    {"Qcache_free_memory", metric.GAUGE},
	"db.qCacheNotCachedPerSecond": {"Qcache_not_cached", metric.PRATE},
	"db.qCacheUtilization":        {qCacheUtilization, metric.GAUGE},
	"db.qCacheHitRatio":           {qCacheHitRatio, metric.GAUGE},
}

func slaveRunningAsNumber(metrics map[string]interface{}, dbVersion string) (int, bool) {
	var prefix string
	if isDBVersionLessThan8Point4(dbVersion) {
		prefix = "Slave"
	} else {
		prefix = "Replica"
	}
	slaveIORunning, okIO := metrics[prefix+"_IO_Running"].(string)
	slaveSQLRunning, okSQL := metrics[prefix+"_SQL_Running"].(string)
	if !okIO || !okSQL {
		return 0, false
	}
	if slaveIORunning == "Yes" && slaveSQLRunning == "Yes" {
		return 1, true
	}
	return 0, true
}

func qCacheUtilization(metrics map[string]interface{}) (float64, bool) {
	// TODO compute the value within the interval
	qCacheFreeBlocks, ok1 := metrics["Qcache_free_blocks"].(int)
	qCacheTotalBlocks, ok2 := metrics["Qcache_total_blocks"].(int)

	if qCacheTotalBlocks == 0 {
		return 0, true
	}

	if ok1 && ok2 {
		return 1 - (float64(qCacheFreeBlocks) / float64(qCacheTotalBlocks)), true
	}
	return 0, false
}

func qCacheHitRatio(metrics map[string]interface{}) (float64, bool) {
	qCacheHits, ok1 := metrics["Qcache_hits"].(int)
	queries, ok2 := metrics["Queries"].(int)

	if queries == 0 {
		return 0, true
	}

	if ok1 && ok2 {
		return float64(qCacheHits) / float64(queries), true
	}
	return 0, false
}

func getDefaultMetrics(dbVersion string) map[string][]interface{} {
	if isDBVersionLessThan8(dbVersion) {
		return mergeMaps(defaultMetricsBase, defaultMetricsBelowVersion8)
	}
	return defaultMetricsBase
}
