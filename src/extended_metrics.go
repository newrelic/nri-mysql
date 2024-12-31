package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var extendedMetricsBase = map[string][]interface{}{
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

var extendedMetricsBelowVersion8 = map[string][]interface{}{
	"db.qCacheFreeBlocks":              {"Qcache_free_blocks", metric.GAUGE},
	"db.qCacheHitsPerSecond":           {"Qcache_hits", metric.RATE},
	"db.qCacheInserts":                 {"Qcache_inserts", metric.GAUGE},
	"db.qCacheLowmemPrunesPerSecond":   {"Qcache_lowmem_prunes", metric.RATE},
	"db.qCacheQueriesInCachePerSecond": {"Qcache_queries_in_cache", metric.RATE},
	"db.qCacheTotalBlocks":             {"Qcache_total_blocks", metric.GAUGE},
}

func threadCacheMissRate(metrics map[string]interface{}) (float64, bool) {
	// TODO compute the value within the interval
	threadsCreated, ok1 := metrics["Threads_created"].(int)
	connections, ok2 := metrics["Connections"].(int)

	if connections == 0 {
		return 0, true
	} else if ok1 && ok2 {
		return float64(threadsCreated) / float64(connections), true
	}
	return 0, false
}

func getExtendedMetrics(dbVersion string) map[string][]interface{} {
	if isDBVersionLessThan8(dbVersion) {
		return mergeMaps(extendedMetricsBase, extendedMetricsBelowVersion8)
	}
	return extendedMetricsBase
}
