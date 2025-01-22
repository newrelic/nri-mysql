package performancemetricscollectors

import (
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// PopulateBlockingSessionMetrics retrieves blocking session metrics from the database and populates them into the integration entity.
func PopulateBlockingSessionMetrics(db utils.DataSource, i *integration.Integration, args arguments.ArgumentList, excludedDatabases []string) {
	// Prepare the SQL query with the provided parameters
	query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, args.QueryCountThreshold)
	if err != nil {
		log.Error("Failed to prepare blocking sessions query: %v", err)
		return
	}

	// Collect the blocking session metrics
	metrics, err := utils.CollectMetrics[utils.BlockingSessionMetrics](db, query, inputArgs...)
	if err != nil {
		log.Error("Error collecting blocking session metrics: %v", err)
		return
	}

	// Return if no metrics are collected
	if len(metrics) == 0 {
		return
	}

	// Set the blocking query metrics in the integration entity
	err = setBlockingQueryMetrics(metrics, i, args)
	if err != nil {
		log.Error("Error setting blocking session metrics: %v", err)
		return
	}
}

// setBlockingQueryMetrics sets the blocking session metrics into the integration entity.
func setBlockingQueryMetrics(metrics []utils.BlockingSessionMetrics, i *integration.Integration, args arguments.ArgumentList) error {
	metricList := make([]interface{}, 0, len(metrics))
	for _, metricData := range metrics {
		metricList = append(metricList, metricData)
	}

	err := utils.IngestMetric(metricList, "MysqlBlockingSessionSample", i, args)
	if err != nil {
		return err
	}
	return nil
}
