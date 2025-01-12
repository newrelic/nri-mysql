package constants

import "time"

const (
	IntegrationName               = "com.newrelic.mysql"
	NodeEntityType                = "node"
	MetricSetLimit                = 100
	ExplainQueryFormat            = "EXPLAIN FORMAT=JSON %s"
	SupportedStatements           = "SELECT INSERT UPDATE DELETE WITH"
	QueryPlanTimeoutDuration      = 10 * time.Second // TimeoutDuration defines the timeout duration for database queries
	TimeoutDuration               = 5 * time.Second  // TimeoutDuration defines the timeout duration for database queries
	MaxQueryCountThreshold        = 30               // MaxQueryCountThreshold defines the maximum number of queries to be collected
	IndividualQueryCountThreshold = 10               // IndividualQueryCountThreshold defines the maximum number of queries to be collected
	MinVersionParts               = 2                // MinVersionParts defines the minimum number of version parts
)

// Default excluded databases
var DefaultExcludedDatabases = []string{"", "mysql", "information_schema", "performance_schema", "sys"}
