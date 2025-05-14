package queryperformancemonitoring

import (
	"fmt"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	dbutils "github.com/newrelic/nri-mysql/src/dbutils"
	infrautils "github.com/newrelic/nri-mysql/src/infrautils"
	performancemetricscollectors "github.com/newrelic/nri-mysql/src/query-performance-monitoring/performance-metrics-collectors"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	validator "github.com/newrelic/nri-mysql/src/query-performance-monitoring/validator"
)

// PopulateQueryPerformanceMetrics serves as the entry point for retrieving and populating query performance metrics, including slow queries, detailed query information, query execution plans, wait events, and blocking sessions.
func PopulateQueryPerformanceMetrics(args arguments.ArgumentList, e *integration.Entity, i *integration.Integration) {
	// Generate Data Source Name (DSN) for database connection
	dsn := dbutils.GenerateDSN(args, "")

	// Open database connection
	db, err := utils.OpenSQLXDB(dsn)
	infrautils.FatalIfErr(err)
	defer db.Close()

	// Validate preconditions before proceeding
	preValidationErr := validator.ValidatePreconditions(db, args)
	if preValidationErr != nil {
		infrautils.FatalIfErr(fmt.Errorf("preconditions failed: %w", preValidationErr))
	}

	// Get the list of unique excluded databases
	excludedDatabases := utils.GetExcludedDatabases(args.ExcludedPerformanceDatabases)

	// Populate metrics for slow queries
	start := time.Now()
	log.Debug("Beginning to retrieve slow query metrics")
	queryIDList := performancemetricscollectors.PopulateSlowQueryMetrics(i, db, args, excludedDatabases)
	log.Debug("Completed fetching slow query metrics in %v", time.Since(start))

	if len(queryIDList) > 0 {
		// Populate metrics for individual queries
		start = time.Now()
		log.Debug("Beginning to retrieve individual query metrics")
		groupQueriesByDatabase, individualQueryDetailsErr := performancemetricscollectors.PopulateIndividualQueryDetails(db, queryIDList, i, args)
		if individualQueryDetailsErr != nil {
			log.Error("Error populating individual query details: %v", individualQueryDetailsErr)
		}
		log.Debug("Completed fetching individual query metrics in %v", time.Since(start))

		if len(groupQueriesByDatabase) > 0 {
			// Populate execution plan details
			start = time.Now()
			log.Debug("Beginning to retrieve query execution plan metrics")
			performancemetricscollectors.PopulateExecutionPlans(db, groupQueriesByDatabase, i, args)
			log.Debug("Completed fetching query execution plan metrics in %v", time.Since(start))
		} else {
			log.Debug("No individual query metrics to fetch.")
		}
	}

	// Populate wait event metrics
	start = time.Now()
	log.Debug("Beginning to retrieve wait event metrics")
	performancemetricscollectors.PopulateWaitEventMetrics(db, i, args, excludedDatabases)
	log.Debug("Completed fetching wait event metrics in %v", time.Since(start))

	// Populate blocking session metrics
	start = time.Now()
	log.Debug("Beginning to retrieve blocking session metrics")
	performancemetricscollectors.PopulateBlockingSessionMetrics(db, i, args, excludedDatabases)
	log.Debug("Completed fetching blocking session metrics in %v", time.Since(start))
	log.Debug("Query analysis completed.")
}
