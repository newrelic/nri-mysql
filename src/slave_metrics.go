package main

import (
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var slaveMetricsBase = map[string][]interface{}{
	"cluster.lastIOErrno":  {"Last_IO_Errno", metric.GAUGE},
	"cluster.lastIOError":  {"Last_IO_Error", metric.ATTRIBUTE},
	"cluster.lastSQLErrno": {"Last_SQL_Errno", metric.GAUGE},
	"cluster.lastSQLError": {"Last_SQL_Error", metric.ATTRIBUTE},
	"db.relayLogSpace":     {"Relay_Log_Space", metric.GAUGE},
}

var slaveMetricsBelowVersion8 = map[string][]interface{}{
	"cluster.secondsBehindMaster": {"Seconds_Behind_Master", metric.GAUGE},
	"cluster.slaveIORunning":      {"Slave_IO_Running", metric.ATTRIBUTE},
	"cluster.slaveSQLRunning":     {"Slave_SQL_Running", metric.ATTRIBUTE},
	"cluster.masterLogFile":       {"Master_Log_File", metric.ATTRIBUTE},
	"cluster.readMasterLogPos":    {"Read_Master_Log_Pos", metric.GAUGE},
	"cluster.relayMasterLogFile":  {"Relay_Master_Log_File", metric.ATTRIBUTE},
	"cluster.execMasterLogPos":    {"Exec_Master_Log_Pos", metric.GAUGE},
	"cluster.masterHost":          {"Master_Host", metric.ATTRIBUTE},
}

var slaveMetricsForVersion8AndAbove = map[string][]interface{}{
	"cluster.secondsBehindMaster": {"Seconds_Behind_Source", metric.GAUGE},
	"cluster.slaveIORunning":      {"Replica_IO_Running", metric.ATTRIBUTE},
	"cluster.slaveSQLRunning":     {"Replica_SQL_Running", metric.ATTRIBUTE},
	"cluster.masterLogFile":       {"Source_Log_File", metric.ATTRIBUTE},
	"cluster.readMasterLogPos":    {"Read_Source_Log_Pos", metric.GAUGE},
	"cluster.relayMasterLogFile":  {"Relay_Source_Log_File", metric.ATTRIBUTE},
	"cluster.execMasterLogPos":    {"Exec_Source_Log_Pos", metric.GAUGE},
	"cluster.masterHost":          {"Source_Host", metric.ATTRIBUTE},
}

// mergeMaps merges two maps, overwriting map1 with any conflicting keys from map2.
func mergeMaps(map1, map2 map[string][]interface{}) map[string][]interface{} {
	for k, v := range map2 {
		map1[k] = v
	}
	return map1
}

func getSlaveMetrics(dbVersion string) map[string][]interface{} {
	// Find the first version definition that's applicable
	if isDBVersionLessThan8(dbVersion) {
		return mergeMaps(slaveMetricsBase, slaveMetricsBelowVersion8)
	}
	return mergeMaps(slaveMetricsBase, slaveMetricsForVersion8AndAbove)
}
