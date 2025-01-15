package constants

import "time"

const (
	IntegrationName     = "com.newrelic.mysql"
	NodeEntityType      = "node"
	MetricSetLimit      = 100
	ExplainQueryFormat  = "EXPLAIN FORMAT=JSON %s"
	SupportedStatements = "SELECT INSERT UPDATE DELETE WITH"
	// QueryPlanTimeoutDuration defines the timeout duration for fetching query plans
	QueryPlanTimeoutDuration = 10 * time.Second
	// TimeoutDuration defines the timeout duration for fetching slow query metrics, individual query metrics, wait event metrics, and blocked session metrics
	TimeoutDuration = 5 * time.Second
	// MaxQueryCountThreshold specifies the upper limit for the number of collected queries, as customers might opt for a higher query count threshold, potentially leading to performance problems.
	MaxQueryCountThreshold = 30
	// IndividualQueryCountThreshold specifies the upper limit for the number of individual queries to be collected, as customers might choose a higher query count threshold, potentially leading to performance problems.
	IndividualQueryCountThreshold = 10
	// MinVersionParts defines the minimum number of version parts
	MinVersionParts = 2
)

// Default excluded databases
var DefaultExcludedDatabases = []string{"", "mysql", "information_schema", "performance_schema", "sys"}
