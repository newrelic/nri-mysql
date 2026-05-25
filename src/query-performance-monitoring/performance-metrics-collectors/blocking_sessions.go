package performancemetricscollectors

import (
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	validator "github.com/newrelic/nri-mysql/src/query-performance-monitoring/validator"
)

// PopulateBlockingSessionMetrics retrieves blocking session metrics from the database and populates them into the integration entity.
func PopulateBlockingSessionMetrics(db utils.DataSource, i *integration.Integration, args arguments.ArgumentList, excludedDatabases []string, querySet utils.QuerySet) {
	// Get the query count threshold
	queryCountThreshold := validator.GetValidQueryCountThreshold(args.QueryMonitoringCountThreshold)

	// Use the appropriate blocking sessions query for the database flavor
	blockingQuery := querySet.BlockingSessionsQuery

	// Prepare the SQL query with the provided parameters
	query, inputArgs, err := sqlx.In(blockingQuery, excludedDatabases, queryCountThreshold)
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

	// Anonymize query texts in Go — only needed for MariaDB, where the trx_query
	// fallback returns raw SQL. MySQL always returns DIGEST_TEXT which is already
	// anonymized by performance_schema.
	if querySet.NeedsQueryAnonymization {
		for i := range metrics {
			metrics[i].BlockedQuery = utils.AnonymizeQueryText(metrics[i].BlockedQuery)
			metrics[i].BlockingQuery = utils.AnonymizeQueryText(metrics[i].BlockingQuery)
		}
	}

	// Set the blocking query metrics in the integration entity and ingest them
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
