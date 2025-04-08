package constants

import "time"

const (
	IntegrationName = "com.newrelic.mysql"
	NodeEntityType  = "node"
	/*
		New Relic's Integration SDK imposes a limit of 1000 metrics per ingestion.
		To handle metric sets exceeding this limit, we process and ingest metrics in smaller chunks
		to ensure all data is successfully reported without exceeding the limit.

		For instance, if QueryCountThreshold is set to 100, then in the worst-case scenario for Individual queries & Query execution plans:
			- Individual queries would amount to 100 * 10 (IndividualQueryCountThreshold), equaling 1000.
			- When considering the execution plan for queries, assuming there are 5 objects in the execution plan JSON for each individual query, this would result in 5000 objects to handle.
	*/
	MetricSetLimit = 600

	/*
		ExplainQueryFormat is a format string used to generate EXPLAIN queries in JSON format.
		Using JSON format simplifies programmatic parsing and analysis of query execution plans.
	*/
	ExplainQueryFormat = "EXPLAIN FORMAT=JSON %s"


	/*
		QueryPlanTimeoutDuration sets the timeout for fetching query execution plans.
		This prevents indefinite waits when a query plan retrieval takes too long, ensuring system responsiveness.
	*/
	QueryPlanTimeoutDuration = 10 * time.Second

	/*
		TimeoutDuration sets a general timeout for various data collection operations (e.g., slow query metrics).
		This prevents long-running operations from causing the integration to hang and ensures timely data retrieval.
	*/
	TimeoutDuration = 5 * time.Second

	// DefaultSlowQueryFetchInterval(sec) defines the default interval for fetching grouped slow query performance metrics. */
	DefaultSlowQueryFetchInterval = 30

	//  DefaultQueryFetchInterval(ms) defines the default interval for fetching individual query performance metrics. */
	DefaultQueryResponseTimeThreshold = 500

	/*
		NOTE: The default and max values chosen may be adjusted in the future. Assumptions made to choose the defaults and max values:

		For instance, if QueryCountThreshold is set to 50, then in the worst-case scenario:
			- Slow queries would total 50.
			- Individual queries would amount to 50 * 10 (IndividualQueryCountThreshold), equaling 500.
			- When considering the execution plan for queries, assuming there are 5 objects in the execution plan JSON for each individual query, this would result in 2500 objects to handle.
			- Wait events would number 50.
			- Blocking sessions would also total 50.

		With a configuration interval set at 30 seconds, processing these results can consume significant time and resources.
	*/

	// DefaultQueryCountThreshold defines the default query count limit for fetching grouped slow, wait events and blocking sessions query performance metrics. */
	DefaultQueryCountThreshold = 20

	/*
		MaxQueryCountThreshold limits the total number of collected queries to prevent performance issues
		that could arise from processing and storing an excessive amount of query data.
		This helps maintain reasonable resource usage by the integration.
	*/
	MaxQueryCountThreshold = 30

	/*
		IndividualQueryCountThreshold limits the number of individual query metrics that are collected.
		This protects system resources by preventing the collection of a potentially overwhelming amount
		of detailed metrics from a large number of unique queries.
	*/
	IndividualQueryCountThreshold = 10

	/*
		MinVersionParts defines the minimum number of parts expected when parsing a version string.
		This ensures version strings are formatted correctly and allows for proper version comparison.
	*/
	MinVersionParts = 2

	/*
		EssentialConsumersCount defines the number of essential consumers that must be enabled
		in the performance schema to ensure that the necessary performance data is available.
	*/
	EssentialConsumersCount = 5
)

/*
DefaultExcludedDatabases defines a list of database names that are excluded by default.
These databases are typically system databases in MySQL that are used for internal purposes
and typically do not require user interactions or modifications.

  - "mysql": This database contains the system user accounts and privileges information.
  - "information_schema": This database provides access to database metadata,
    i.e., data about data. It is read-only and is used for querying about database objects.
  - "performance_schema": This database provides performance-related data and metrics
    about server execution and resource usage. It is mainly used for monitoring purposes.
  - "sys": This database provides simplified views and functions for easier system
    administration and performance tuning.
  - "": The empty string is included because some queries may not be associated with
    any specific database. Including "" ensures that these undetermined or global queries
    are not incorrectly related to a specific user database.

Excluding these databases by default helps to prevent accidental modifications
and focuses system operations only on user-defined databases.
*/
var DefaultExcludedDatabases = []string{"", "mysql", "information_schema", "performance_schema", "sys"}
