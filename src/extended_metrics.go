package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var extendedMetricsBase = map[string][]interface{}{
	"db.createdTmpDiskTablesPerSecond":     {"Created_tmp_disk_tables", metric.PRATE},
	"db.createdTmpFilesPerSecond":          {"Created_tmp_files", metric.PRATE},
	"db.createdTmpTablesPerSecond":         {"Created_tmp_tables", metric.PRATE},
	"db.handlerDeletePerSecond":            {"Handler_delete", metric.PRATE},
	"db.handlerReadFirstPerSecond":         {"Handler_read_first", metric.PRATE},
	"db.handlerReadKeyPerSecond":           {"Handler_read_key", metric.PRATE},
	"db.handlerReadRndPerSecond":           {"Handler_read_rnd", metric.PRATE},
	"db.handlerReadRndNextPerSecond":       {"Handler_read_rnd_next", metric.PRATE},
	"db.handlerUpdatePerSecond":            {"Handler_update", metric.PRATE},
	"db.handlerWritePerSecond":             {"Handler_write", metric.PRATE},
	"db.maxExecutionTimeExceededPerSecond": {"Max_execution_time_exceeded", metric.PRATE},
	"db.selectFullJoinPerSecond":           {"Select_full_join", metric.PRATE},
	"db.selectFullJoinRangePerSecond":      {"Select_full_range_join", metric.PRATE},
	"db.selectRangePerSecond":              {"Select_range", metric.PRATE},
	"db.selectRangeCheckPerSecond":         {"Select_range_check", metric.PRATE},
	"db.sortMergePassesPerSecond":          {"Sort_merge_passes", metric.PRATE},
	"db.sortRangePerSecond":                {"Sort_range", metric.PRATE},
	"db.sortRowsPerSecond":                 {"Sort_rows", metric.PRATE},
	"db.sortScanPerSecond":                 {"Sort_scan", metric.PRATE},
	"db.tableOpenCacheHitsPerSecond":       {"Table_open_cache_hits", metric.PRATE},
	"db.tableOpenCacheMissesPerSecond":     {"Table_open_cache_misses", metric.PRATE},
	"db.tableOpenCacheOverflowsPerSecond":  {"Table_open_cache_overflows", metric.PRATE},
	"db.threadsCached":                     {"Threads_cached", metric.GAUGE},
	"db.threadsCreatedPerSecond":           {"Threads_created", metric.PRATE},
	"db.threadCacheMissRate":               {threadCacheMissRate, metric.GAUGE},
}

var extendedMetricsBelowVersion8 = map[string][]interface{}{
	"db.qCacheFreeBlocks":              {"Qcache_free_blocks", metric.GAUGE},
	"db.qCacheHitsPerSecond":           {"Qcache_hits", metric.PRATE},
	"db.qCacheInserts":                 {"Qcache_inserts", metric.GAUGE},
	"db.qCacheLowmemPrunesPerSecond":   {"Qcache_lowmem_prunes", metric.PRATE},
	"db.qCacheQueriesInCachePerSecond": {"Qcache_queries_in_cache", metric.PRATE},
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
