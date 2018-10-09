package main

import (
	"github.com/newrelic/infra-integrations-sdk/metric"
)

var defaultMetrics = map[string][]interface{}{
	"net.abortedClientsPerSecond":                 {"Aborted_clients", metric.RATE},
	"net.abortedConnectsPerSecond":                {"Aborted_connects", metric.RATE},
	"net.bytesReceivedPerSecond":                  {"Bytes_received", metric.RATE},
	"net.bytesSentPerSecond":                      {"Bytes_sent", metric.RATE},
	"net.connectionErrorsMaxConnectionsPerSecond": {"Connection_errors_max_connections", metric.RATE},
	"net.connectionsPerSecond":                    {"Connections", metric.RATE},
	"net.maxUsedConnections":                      {"Max_used_connections", metric.GAUGE},
	"net.threadsConnected":                        {"Threads_connected", metric.GAUGE},
	"net.threadsRunning":                          {"Threads_running", metric.GAUGE},
	"query.comDeletePerSecond":                    {"Com_delete", metric.RATE},
	"query.comDeleteMultiPerSecond":               {"Com_delete_multi", metric.RATE},
	"query.comInsertPerSecond":                    {"Com_insert", metric.RATE},
	"query.comInsertSelectPerSecond":              {"Com_insert_select", metric.RATE},
	"query.comReplaceSelectPerSecond":             {"Com_replace_select", metric.RATE},
	"query.comSelectPerSecond":                    {"Com_select", metric.RATE},
	"query.comUpdatePerSecond":                    {"Com_update", metric.RATE},
	"query.comUpdateMultiPerSecond":               {"Com_update_multi", metric.RATE},
	"db.handlerRollbackPerSecond":                 {"Handler_rollback", metric.RATE},
	"query.preparedStmtCountPerSecond":            {"Prepared_stmt_count", metric.RATE},
	"query.queriesPerSecond":                      {"Queries", metric.RATE},
	"query.questionsPerSecond":                    {"Questions", metric.RATE},
	"query.slowQueriesPerSecond":                  {"Slow_queries", metric.RATE},
	"db.innodb.bufferPoolPagesData":               {"Innodb_buffer_pool_pages_data", metric.GAUGE},
	"db.innodb.bufferPoolPagesFree":               {"Innodb_buffer_pool_pages_free", metric.GAUGE},
	"db.innodb.bufferPoolPagesTotal":              {"Innodb_buffer_pool_pages_total", metric.GAUGE},
	"db.innodb.dataReadBytesPerSecond":            {"Innodb_data_read", metric.RATE},
	"db.innodb.dataWrittenBytesPerSecond":         {"Innodb_data_written", metric.RATE},
	"db.innodb.logWaitsPerSecond":                 {"Innodb_log_waits", metric.RATE},
	"db.innodb.rowLockCurrentWaits":               {"Innodb_row_lock_current_waits", metric.GAUGE},
	"db.innodb.rowLockTimeAvg":                    {"Innodb_row_lock_time_avg", metric.GAUGE},
	"db.innodb.rowLockWaitsPerSecond":             {"Innodb_row_lock_waits", metric.RATE},
	"db.openFiles":                                {"Open_files", metric.GAUGE},
	"db.openTables":                               {"Open_tables", metric.GAUGE},
	"db.openedTablesPerSecond":                    {"Opened_tables", metric.RATE},
	"db.qCacheFreeMemoryBytes":                    {"Qcache_free_memory", metric.GAUGE},
	"db.qCacheNotCachedPerSecond":                 {"Qcache_not_cached", metric.RATE},
	"db.tablesLocksWaitedPerSecond":               {"Table_locks_waited", metric.RATE},
	"db.qCacheUtilization":                        {qCacheUtilization, metric.GAUGE},
	"db.qCacheHitRatio":                           {qCacheHitRatio, metric.GAUGE},
	"software.edition":                            {"version_comment", metric.ATTRIBUTE},
	"software.version":                            {"version", metric.ATTRIBUTE},
	"cluster.nodeType":                            {"node_type", metric.ATTRIBUTE},
	// If a cluster instance is not a slave, then the metric cluster.slaveRunning will be removed.
	"cluster.slaveRunning": {slaveRunningAsNumber, metric.GAUGE},
}

func qCacheUtilization(metrics map[string]interface{}) (float64, bool) {
	//TODO compute the value within the interval
	qCacheFreeBlocks, ok1 := metrics["Qcache_free_blocks"].(int)
	qCacheTotalBlocks, ok2 := metrics["Qcache_total_blocks"].(int)

	if qCacheTotalBlocks == 0 {
		return 0, true
	} else if ok1 && ok2 {
		return 1 - (float64(qCacheFreeBlocks) / float64(qCacheTotalBlocks)), true
	}
	return 0, false
}

func qCacheHitRatio(metrics map[string]interface{}) (float64, bool) {
	qCacheHits, ok1 := metrics["Qcache_hits"].(int)
	queries, ok2 := metrics["Queries"].(int)

	if queries == 0 {
		return 0, true
	} else if ok1 && ok2 {
		return float64(qCacheHits) / float64(queries), true
	}
	return 0, false
}

func slaveRunningAsNumber(metrics map[string]interface{}) (int, bool) {
	slaveRunning, ok := metrics["Slave_running"].(string)
	if ok && slaveRunning == "ON" {
		return 1, true
	} else if ok && slaveRunning == "OFF" {
		return 0, true
	}
	return 0, false
}

var extendedMetrics = map[string][]interface{}{
	"db.createdTmpDiskTablesPerSecond":     {"Created_tmp_disk_tables", metric.RATE},
	"db.createdTmpFilesPerSecond":          {"Created_tmp_files", metric.RATE},
	"db.createdTmpTablesPerSecond":         {"Created_tmp_tables", metric.RATE},
	"db.handlerDeletePerSecond":            {"Handler_delete", metric.RATE},
	"db.handlerReadFirstPerSecond":         {"Handler_read_first", metric.RATE},
	"db.handlerReadKeyPerSecond":           {"Handler_read_key", metric.RATE},
	"db.handlerReadRndPerSecond":           {"Handler_read_rnd", metric.RATE},
	"db.handlerReadRndNextPerSecond":       {"Handler_read_rnd_next", metric.RATE},
	"db.handlerUpdatePerSecond":            {"Handler_update", metric.RATE},
	"db.handlerWritePerSecond":             {"Handler_write", metric.RATE},
	"db.maxExecutionTimeExceededPerSecond": {"Max_execution_time_exceeded", metric.RATE},
	"db.qCacheFreeBlocks":                  {"Qcache_free_blocks", metric.GAUGE},
	"db.qCacheHitsPerSecond":               {"Qcache_hits", metric.RATE},
	"db.qCacheInserts":                     {"Qcache_inserts", metric.GAUGE},
	"db.qCacheLowmemPrunesPerSecond":       {"Qcache_lowmem_prunes", metric.RATE},
	"db.qCacheQueriesInCachePerSecond":     {"Qcache_queries_in_cache", metric.RATE},
	"db.qCacheTotalBlocks":                 {"Qcache_total_blocks", metric.GAUGE},
	"db.selectFullJoinPerSecond":           {"Select_full_join", metric.RATE},
	"db.selectFullJoinRangePerSecond":      {"Select_full_range_join", metric.RATE},
	"db.selectRangePerSecond":              {"Select_range", metric.RATE},
	"db.selectRangeCheckPerSecond":         {"Select_range_check", metric.RATE},
	"db.sortMergePassesPerSecond":          {"Sort_merge_passes", metric.RATE},
	"db.sortRangePerSecond":                {"Sort_range", metric.RATE},
	"db.sortRowsPerSecond":                 {"Sort_rows", metric.RATE},
	"db.sortScanPerSecond":                 {"Sort_scan", metric.RATE},
	"db.tableOpenCacheHitsPerSecond":       {"Table_open_cache_hits", metric.RATE},
	"db.tableOpenCacheMissesPerSecond":     {"Table_open_cache_misses", metric.RATE},
	"db.tableOpenCacheOverflowsPerSecond":  {"Table_open_cache_overflows", metric.RATE},
	"db.threadsCached":                     {"Threads_cached", metric.GAUGE},
	"db.threadsCreatedPerSecond":           {"Threads_created", metric.RATE},
	"db.threadCacheMissRate":               {threadCacheMissRate, metric.GAUGE},
}

func threadCacheMissRate(metrics map[string]interface{}) (float64, bool) {
	//TODO compute the value within the interval
	threadsCreated, ok1 := metrics["Threads_created"].(int)
	connections, ok2 := metrics["Connections"].(int)

	if connections == 0 {
		return 0, true
	} else if ok1 && ok2 {
		return float64(threadsCreated) / float64(connections), true
	}
	return 0, false
}

var slaveMetrics = map[string][]interface{}{
	"cluster.secondsBehindMaster": {"Seconds_Behind_Master", metric.GAUGE},
	"cluster.slaveIORunning":      {"Slave_IO_Running", metric.ATTRIBUTE},
	"cluster.slaveSQLRunning":     {"Slave_SQL_Running", metric.ATTRIBUTE},
	"cluster.lastIOErrno":         {"Last_IO_Errno", metric.GAUGE},
	"cluster.lastIOError":         {"Last_IO_Error", metric.ATTRIBUTE},
	"cluster.lastSQLErrno":        {"Last_SQL_Errno", metric.GAUGE},
	"cluster.lastSQLError":        {"Last_SQL_Error", metric.ATTRIBUTE},
	"cluster.masterLogFile":       {"Master_Log_File", metric.ATTRIBUTE},
	"cluster.readMasterLogPos":    {"Read_Master_Log_Pos", metric.GAUGE},
	"cluster.relayMasterLogFile":  {"Relay_Master_Log_File", metric.ATTRIBUTE},
	"cluster.execMasterLogPos":    {"Exec_Master_Log_Pos", metric.GAUGE},
	"db.relayLogSpace":            {"Relay_Log_Space", metric.GAUGE},
	"cluster.masterHost":          {"Master_Host", metric.ATTRIBUTE},
}

var innodbMetrics = map[string][]interface{}{
	"db.innodb.bufferPoolPagesDirty":                {"Innodb_buffer_pool_pages_dirty", metric.GAUGE},
	"db.innodb.bufferPoolPagesFlushedPerSecond":     {"Innodb_buffer_pool_pages_flushed", metric.RATE},
	"db.innodb.bufferPoolReadAheadPerSecond":        {"Innodb_buffer_pool_read_ahead", metric.RATE},
	"db.innodb.bufferPoolReadAheadEvictedPerSecond": {"Innodb_buffer_pool_read_ahead_evicted", metric.RATE},
	"db.innodb.bufferPoolReadAheadRndPerSecond":     {"Innodb_buffer_pool_read_ahead_rnd", metric.RATE},
	"db.innodb.bufferPoolReadRequestsPerSecond":     {"Innodb_buffer_pool_read_requests", metric.RATE},
	"db.innodb.bufferPoolReadsPerSecond":            {"Innodb_buffer_pool_reads", metric.RATE},
	"db.innodb.bufferPoolWaitFreePerSecond":         {"Innodb_buffer_pool_wait_free", metric.RATE},
	"db.innodb.bufferPoolWriteRequestsPerSecond":    {"Innodb_buffer_pool_write_requests", metric.RATE},
	"db.innodb.dataFsyncsPerSecond":                 {"Innodb_data_fsyncs", metric.RATE},
	"db.innodb.dataPendingFsyncs":                   {"Innodb_data_pending_fsyncs", metric.GAUGE},
	"db.innodb.dataPendingReads":                    {"Innodb_data_pending_reads", metric.GAUGE},
	"db.innodb.dataPendingWrites":                   {"Innodb_data_pending_writes", metric.GAUGE},
	"db.innodb.dataReadsPerSecond":                  {"Innodb_data_reads", metric.RATE},
	"db.innodb.dataWritesPerSecond":                 {"Innodb_data_writes", metric.RATE},
	"db.innodb.logWriteRequestsPerSecond":           {"Innodb_log_write_requests", metric.RATE},
	"db.innodb.logWritesPerSecond":                  {"Innodb_log_writes", metric.RATE},
	"db.innodb.numOpenFiles":                        {"Innodb_num_open_files", metric.GAUGE},
	"db.innodb.osLogFsyncsPerSecond":                {"Innodb_os_log_fsyncs", metric.RATE},
	"db.innodb.osLogPendingFsyncs":                  {"Innodb_os_log_pending_fsyncs", metric.GAUGE},
	"db.innodb.osLogPendingWrites":                  {"Innodb_os_log_pending_writes", metric.GAUGE},
	"db.innodb.osLogWrittenBytesPerSecond":          {"Innodb_os_log_written", metric.RATE},
	"db.innodb.pagesCreatedPerSecond":               {"Innodb_pages_created", metric.RATE},
	"db.innodb.pagesReadPerSecond":                  {"Innodb_pages_read", metric.RATE},
	"db.innodb.pagesWrittenPerSecond":               {"Innodb_pages_written", metric.RATE},
	"db.innodb.rowsDeletedPerSecond":                {"Innodb_rows_deleted", metric.RATE},
	"db.innodb.rowsInsertedPerSecond":               {"Innodb_rows_inserted", metric.RATE},
	"db.innodb.rowsReadPerSecond":                   {"Innodb_rows_read", metric.RATE},
	"db.innodb.rowsUpdatedPerSecond":                {"Innodb_rows_updated", metric.RATE},
}

var myisamMetrics = map[string][]interface{}{
	"db.myisam.keyBlocksNotFlushed":       {"Key_blocks_not_flushed", metric.GAUGE},
	"db.myisam.keyCacheUtilization":       {keyCacheUtilization, metric.GAUGE},
	"db.myisam.keyReadRequestsPerSecond":  {"Key_read_requests", metric.RATE},
	"db.myisam.keyReadsPerSecond":         {"Key_reads", metric.RATE},
	"db.myisam.keyWriteRequestsPerSecond": {"Key_write_requests", metric.RATE},
	"db.myisam.keyWritesPerSecond":        {"Key_writes", metric.RATE},
}

func keyCacheUtilization(metrics map[string]interface{}) (float64, bool) {
	keyBlocksUnused, ok1 := metrics["Key_blocks_unused"].(int)
	keyCacheBlockSize, ok2 := metrics["key_cache_block_size"].(int)
	keyBufferSize, ok3 := metrics["key_buffer_size"].(int)

	if keyBufferSize == 0 {
		return 0, true
	} else if ok1 && ok2 && ok3 {
		return 1 - (float64(keyBlocksUnused) * float64(keyCacheBlockSize) / float64(keyBufferSize)), true
	}
	return 0, false
}
