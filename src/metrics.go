package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var innodbMetrics = map[string][]interface{}{
	"db.innodb.bufferPoolPagesDirty":                {"Innodb_buffer_pool_pages_dirty", metric.GAUGE},
	"db.innodb.bufferPoolPagesFlushedPerSecond":     {"Innodb_buffer_pool_pages_flushed", metric.PRATE},
	"db.innodb.bufferPoolReadAheadPerSecond":        {"Innodb_buffer_pool_read_ahead", metric.PRATE},
	"db.innodb.bufferPoolReadAheadEvictedPerSecond": {"Innodb_buffer_pool_read_ahead_evicted", metric.PRATE},
	"db.innodb.bufferPoolReadAheadRndPerSecond":     {"Innodb_buffer_pool_read_ahead_rnd", metric.PRATE},
	"db.innodb.bufferPoolReadRequestsPerSecond":     {"Innodb_buffer_pool_read_requests", metric.PRATE},
	"db.innodb.bufferPoolReadsPerSecond":            {"Innodb_buffer_pool_reads", metric.PRATE},
	"db.innodb.bufferPoolWaitFreePerSecond":         {"Innodb_buffer_pool_wait_free", metric.PRATE},
	"db.innodb.bufferPoolWriteRequestsPerSecond":    {"Innodb_buffer_pool_write_requests", metric.PRATE},
	"db.innodb.dataFsyncsPerSecond":                 {"Innodb_data_fsyncs", metric.PRATE},
	"db.innodb.dataPendingFsyncs":                   {"Innodb_data_pending_fsyncs", metric.GAUGE},
	"db.innodb.dataPendingReads":                    {"Innodb_data_pending_reads", metric.GAUGE},
	"db.innodb.dataPendingWrites":                   {"Innodb_data_pending_writes", metric.GAUGE},
	"db.innodb.dataReadsPerSecond":                  {"Innodb_data_reads", metric.PRATE},
	"db.innodb.dataWritesPerSecond":                 {"Innodb_data_writes", metric.PRATE},
	"db.innodb.logWriteRequestsPerSecond":           {"Innodb_log_write_requests", metric.PRATE},
	"db.innodb.logWritesPerSecond":                  {"Innodb_log_writes", metric.PRATE},
	"db.innodb.numOpenFiles":                        {"Innodb_num_open_files", metric.GAUGE},
	"db.innodb.osLogFsyncsPerSecond":                {"Innodb_os_log_fsyncs", metric.PRATE},
	"db.innodb.osLogPendingFsyncs":                  {"Innodb_os_log_pending_fsyncs", metric.GAUGE},
	"db.innodb.osLogPendingWrites":                  {"Innodb_os_log_pending_writes", metric.GAUGE},
	"db.innodb.osLogWrittenBytesPerSecond":          {"Innodb_os_log_written", metric.PRATE},
	"db.innodb.pagesCreatedPerSecond":               {"Innodb_pages_created", metric.PRATE},
	"db.innodb.pagesReadPerSecond":                  {"Innodb_pages_read", metric.PRATE},
	"db.innodb.pagesWrittenPerSecond":               {"Innodb_pages_written", metric.PRATE},
	"db.innodb.rowsDeletedPerSecond":                {"Innodb_rows_deleted", metric.PRATE},
	"db.innodb.rowsInsertedPerSecond":               {"Innodb_rows_inserted", metric.PRATE},
	"db.innodb.rowsReadPerSecond":                   {"Innodb_rows_read", metric.PRATE},
	"db.innodb.rowsUpdatedPerSecond":                {"Innodb_rows_updated", metric.PRATE},
}

var myisamMetrics = map[string][]interface{}{
	"db.myisam.keyBlocksNotFlushed":       {"Key_blocks_not_flushed", metric.GAUGE},
	"db.myisam.keyCacheUtilization":       {keyCacheUtilization, metric.GAUGE},
	"db.myisam.keyReadRequestsPerSecond":  {"Key_read_requests", metric.PRATE},
	"db.myisam.keyReadsPerSecond":         {"Key_reads", metric.PRATE},
	"db.myisam.keyWriteRequestsPerSecond": {"Key_write_requests", metric.PRATE},
	"db.myisam.keyWritesPerSecond":        {"Key_writes", metric.PRATE},
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
