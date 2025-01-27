package performancemetricscollectors

import (
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	validator "github.com/newrelic/nri-mysql/src/query-performance-monitoring/validator"
)

// PopulateWaitEventMetrics retrieves wait event metrics from the database and sets them in the integration.
func PopulateWaitEventMetrics(db utils.DataSource, i *integration.Integration, args arguments.ArgumentList, excludedDatabases []string) {
	// Get the query count threshold
	queryCountThreshold := validator.GetValidQueryCountThreshold(args.QueryCountThreshold)

	// Prepare the arguments for the query
	excludedDatabasesArgs := []interface{}{excludedDatabases, excludedDatabases, queryCountThreshold}

	// Prepare the SQL query with the provided parameters
	preparedQuery, preparedArgs, err := sqlx.In(utils.WaitEventsQuery, excludedDatabasesArgs...)
	if err != nil {
		log.Error("Failed to prepare wait event query: %v", err)
		return
	}

	// Collect the wait event metrics
	metrics, err := utils.CollectMetrics[utils.WaitEventQueryMetrics](db, preparedQuery, preparedArgs...)
	if err != nil {
		log.Error("Error collecting wait event metrics: %v", err)
		return
	}

	// Return if no metrics are collected
	if len(metrics) == 0 {
		return
	}
	// Set the retrieved metrics in the integration
	err = setWaitEventMetrics(i, args, metrics)
	if err != nil {
		log.Error("Error setting wait event metrics: %v", err)
		return
	}
}

// setWaitEventMetrics sets the wait event metrics in the integration.
func setWaitEventMetrics(i *integration.Integration, args arguments.ArgumentList, metrics []utils.WaitEventQueryMetrics) error {
	metricList := make([]interface{}, 0, len(metrics))
	for _, metricData := range metrics {
		metricList = append(metricList, metricData)
	}

	err := utils.IngestMetric(metricList, "MysqlWaitEventsSample", i, args)
	if err != nil {
		return err
	}
	return nil
}
