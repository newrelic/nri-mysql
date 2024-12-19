package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

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
