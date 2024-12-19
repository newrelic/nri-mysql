package main

import (
	"github.com/blang/semver/v4"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
)

var slaveMetricsBase = map[string][]interface{}{
	"cluster.lastIOErrno":  {"Last_IO_Errno", metric.GAUGE},
	"cluster.lastIOError":  {"Last_IO_Error", metric.ATTRIBUTE},
	"cluster.lastSQLErrno": {"Last_SQL_Errno", metric.GAUGE},
	"cluster.lastSQLError": {"Last_SQL_Error", metric.ATTRIBUTE},
	"db.relayLogSpace":     {"Relay_Log_Space", metric.GAUGE},
}

var slaveMetrics560 = map[string][]interface{}{
	"cluster.secondsBehindMaster": {"Seconds_Behind_Master", metric.GAUGE},
	"cluster.slaveIORunning":      {"Slave_IO_Running", metric.ATTRIBUTE},
	"cluster.slaveSQLRunning":     {"Slave_SQL_Running", metric.ATTRIBUTE},
	"cluster.masterLogFile":       {"Master_Log_File", metric.ATTRIBUTE},
	"cluster.readMasterLogPos":    {"Read_Master_Log_Pos", metric.GAUGE},
	"cluster.relayMasterLogFile":  {"Relay_Master_Log_File", metric.ATTRIBUTE},
	"cluster.execMasterLogPos":    {"Exec_Master_Log_Pos", metric.GAUGE},
	"cluster.masterHost":          {"Master_Host", metric.ATTRIBUTE},
}

var slaveMetrics800 = map[string][]interface{}{
	"cluster.secondsBehindMaster": {"Seconds_Behind_Source", metric.GAUGE},
	"cluster.slaveIORunning":      {"Replica_IO_Running", metric.ATTRIBUTE},
	"cluster.slaveSQLRunning":     {"Replica_SQL_Running", metric.ATTRIBUTE},
	"cluster.masterLogFile":       {"Source_Log_File", metric.ATTRIBUTE},
	"cluster.readMasterLogPos":    {"Read_Source_Log_Pos", metric.GAUGE},
	"cluster.relayMasterLogFile":  {"Relay_Source_Log_File", metric.ATTRIBUTE},
	"cluster.execMasterLogPos":    {"Exec_Source_Log_Pos", metric.GAUGE},
	"cluster.masterHost":          {"Source_Host", metric.ATTRIBUTE},
}

type VersionDefinition struct {
	minVersion        semver.Version
	metricsDefinition map[string][]interface{}
}

var versionDefinitions = []VersionDefinition{
	{
		minVersion:        semver.MustParse("8.0.0"),
		metricsDefinition: mergeMaps(slaveMetricsBase, slaveMetrics800),
	},
	{
		minVersion:        semver.MustParse("5.6.0"),
		metricsDefinition: mergeMaps(slaveMetricsBase, slaveMetrics560),
	},
}

// mergeMaps merges two maps of type map[string][]interface{}
func mergeMaps(map1, map2 map[string][]interface{}) map[string][]interface{} {
	merged := make(map[string][]interface{})
	for k, v := range map1 {
		merged[k] = v
	}
	for k, v := range map2 {
		merged[k] = v
	}
	return merged
}

func getSlaveMetrics(version *semver.Version) map[string][]interface{} {
	// Find the first version definition that's applicable
	for _, versionDef := range versionDefinitions {
		if version.GE(versionDef.minVersion) {
			return versionDef.metricsDefinition
		}
	}
	return slaveMetricsBase
}
