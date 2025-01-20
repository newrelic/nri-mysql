package queryperformancemonitoring

import (
	"fmt"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	performancemetricscollectors "github.com/newrelic/nri-mysql/src/query-performance-monitoring/performance-metrics-collectors"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	validator "github.com/newrelic/nri-mysql/src/query-performance-monitoring/validator"
)

// main
func PopulateQueryPerformanceMetrics(args arguments.ArgumentList, e *integration.Entity, i *integration.Integration) {
	var database string

	// Generate Data Source Name (DSN) for database connection
	dsn := utils.GenerateDSN(args, database)

	// Open database connection
	db, err := utils.OpenDB(dsn)
	utils.FatalIfErr(err)
	defer db.Close()

	// Validate preconditions before proceeding
	preValidationErr := validator.ValidatePreconditions(db)
	if preValidationErr != nil {
		utils.FatalIfErr(fmt.Errorf("preconditions failed: %w", preValidationErr))
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
		groupQueriesByDatabase := performancemetricscollectors.PopulateIndividualQueryDetails(db, queryIDList, i, args)
		log.Debug("Completed fetching individual query metrics in %v", time.Since(start))

		// Populate execution plan details
		start = time.Now()
		log.Debug("Beginning to retrieve query execution plan metrics")
		performancemetricscollectors.PopulateExecutionPlans(db, groupQueriesByDatabase, i, args)
		log.Debug("Completed fetching query execution plan metrics in %v", time.Since(start))
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
