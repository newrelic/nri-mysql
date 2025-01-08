package constants

import "time"

const (
	IntegrationName               = "com.newrelic.mysql"
	NodeEntityType                = "node"
	MetricSetLimit                = 100
	ExplainQueryFormat            = "EXPLAIN FORMAT=JSON %s"
	SupportedStatements           = "SELECT INSERT UPDATE DELETE WITH"
	QueryPlanTimeoutDuration      = 10 * time.Second
	TimeoutDuration               = 5 * time.Second // TimeoutDuration defines the timeout duration for database queries
	MaxQueryCountThreshold        = 30
	IndividualQueryCountThreshold = 10
	MinVersionParts               = 2
)
