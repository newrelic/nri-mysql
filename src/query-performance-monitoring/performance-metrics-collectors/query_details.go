package performancemetricscollectors

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	validator "github.com/newrelic/nri-mysql/src/query-performance-monitoring/validator"
)

// PopulateSlowQueryMetrics collects and sets slow query metrics and returns the list of query IDs
func PopulateSlowQueryMetrics(i *integration.Integration, db utils.DataSource, args arguments.ArgumentList, excludedDatabases []string) []string {
	// Get the slow query fetch interval
	slowQueryFetchInterval := validator.GetValidSlowQueryFetchIntervalThreshold(args.SlowQueryFetchInterval)

	// Get the query count threshold
	queryCountThreshold := validator.GetValidQueryCountThreshold(args.QueryMonitoringCountThreshold)

	rawMetrics, queryIDList, err := collectGroupedSlowQueryMetrics(db, slowQueryFetchInterval, queryCountThreshold, excludedDatabases)
	if err != nil {
		log.Error("Failed to collect slow query metrics: %v", err)
		return []string{}
	}

	// Return if no metrics are collected
	if len(rawMetrics) == 0 {
		return []string{}
	}

	err = setSlowQueryMetrics(i, rawMetrics, args)
	if err != nil {
		log.Error("Failed to set slow query metrics: %v", err)
		return []string{}
	}

	return queryIDList
}

// collectGroupedSlowQueryMetrics collects metrics from the performance schema database for slow queries
func collectGroupedSlowQueryMetrics(db utils.DataSource, slowQueryfetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.SlowQueryMetrics, []string, error) {
	// Prepare the SQL query with the provided parameters
	query, args, err := sqlx.In(utils.SlowQueries, slowQueryfetchInterval, excludedDatabases, queryCountThreshold)
	if err != nil {
		return nil, []string{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutDuration)
	defer cancel()
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, []string{}, err
	}
	defer rows.Close()

	var metrics []utils.SlowQueryMetrics
	var qIDList []string
	for rows.Next() {
		var metric utils.SlowQueryMetrics
		var qID string
		if err := rows.StructScan(&metric); err != nil {
			return nil, []string{}, err
		}
		if metric.QueryID == nil {
			log.Warn("Query ID is nil for metric: %v. Skipping metric collection. This is an issue because Query ID is required to uniquely identify the query being collected.", metric)
			continue
		}
		qID = *metric.QueryID
		qIDList = append(qIDList, qID)
		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, []string{}, err
	}

	return metrics, qIDList, nil
}

// setSlowQueryMetrics sets the collected slow query metrics to the integration
func setSlowQueryMetrics(i *integration.Integration, metrics []utils.SlowQueryMetrics, args arguments.ArgumentList) error {
	metricList := make([]interface{}, 0, len(metrics))
	for _, metricData := range metrics {
		metricList = append(metricList, metricData)
	}

	err := utils.IngestMetric(metricList, "MysqlSlowQueriesSample", i, args)
	if err != nil {
		return err
	}
	return nil
}

// PopulateIndividualQueryDetails collects and sets individual query details
func PopulateIndividualQueryDetails(db utils.DataSource, queryIDList []string, i *integration.Integration, args arguments.ArgumentList) (map[string][]utils.IndividualQueryMetrics, error) {
	// Retrieve the list of individual queries with combined metrics
	queryList, err := getIndividualQueryList(db, queryIDList, args)
	if err != nil {
		log.Error("Failed to collect query metrics: %v", err)
		return nil, err
	}

	// Prepare a copy of the query list for reporting
	queryListPatchedCopy := setupQueryListCopyForReporting(queryList)

	// Ingest the patched copy of the query list for reporting
	if err := utils.IngestMetric(queryListPatchedCopy, "MysqlIndividualQueriesSample", i, args); err != nil {
		log.Error("Failed to ingest individual query metrics: %v", err)
		return nil, err
	}

	// Group queries by database for further use or reporting
	groupQueriesByDatabase := groupQueriesByDatabase(queryList)

	return groupQueriesByDatabase, nil
}

// getIndividualQueryList fetches and combines current, recent, and extensive query metrics
func getIndividualQueryList(db utils.DataSource, queryIDList []string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	// Collect current query metrics from the performance schema database for the given query IDs
	currentQueryMetrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.CurrentRunningQueriesSearch, args)
	if err != nil {
		return nil, fmt.Errorf("failed to collect current query metrics: %w", err)
	}

	// Collect recent query metrics from the performance schema database for the given query IDs
	recentQueryMetrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.RecentQueriesSearch, args)
	if err != nil {
		return nil, fmt.Errorf("failed to collect recent query metrics: %w", err)
	}

	// Collect extensive query metrics from the performance schema database for the given query IDs
	extensiveQueryMetrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.PastQueriesSearch, args)
	if err != nil {
		return nil, fmt.Errorf("failed to collect extensive query metrics: %w", err)
	}

	// Combine all collected metrics into a single list
	var allMetrics []utils.IndividualQueryMetrics
	allMetrics = append(allMetrics, currentQueryMetrics...)
	allMetrics = append(allMetrics, recentQueryMetrics...)
	allMetrics = append(allMetrics, extensiveQueryMetrics...)

	return allMetrics, nil
}

// setupQueryListCopyForReporting prepares the query list by removing unnecessary data
func setupQueryListCopyForReporting(originalQueryList []utils.IndividualQueryMetrics) []interface{} {
	// Create a new list to hold the modified query metrics
	modifiedQueryList := make([]utils.IndividualQueryMetrics, len(originalQueryList))

	// Copy the original query list to the new list
	copy(modifiedQueryList, originalQueryList)

	// Create a list of interfaces to hold the modified metrics for ingestion
	metricsForIngestion := make([]interface{}, 0, len(modifiedQueryList))

	// Iterate over the modified query list and remove the QueryText field from each metric
	for i := range modifiedQueryList {
		// Exclude QueryText from ingestion as it is only used for fetching the query execution plan
		modifiedQueryList[i].QueryText = nil

		// Add the modified query metric to the list for ingestion
		metricsForIngestion = append(metricsForIngestion, modifiedQueryList[i])
	}

	// Return the list of metrics for ingestion
	return metricsForIngestion
}

// groupQueriesByDatabase groups a list of IndividualQueryMetrics by their database names.
// It returns a map where the keys are database names and the values are slices of IndividualQueryMetrics
// that belong to those databases.
func groupQueriesByDatabase(filteredList []utils.IndividualQueryMetrics) map[string][]utils.IndividualQueryMetrics {
	groupMap := make(map[string][]utils.IndividualQueryMetrics)

	for _, query := range filteredList {
		// Check if the database name is nil
		if query.DatabaseName == nil {
			// Log a warning if the query text is not nil
			if query.QueryText != nil {
				log.Warn("Skipping query with nil database name: QueryText=%v", *query.QueryText)
			} else {
				// Log a warning if both the database name and query text are nil
				log.Warn("Skipping query with nil database name and nil query text")
			}
			continue
		}
		// Check if the query text is nil
		if query.QueryText == nil {
			// Log a warning if the query text is nil
			log.Warn("Skipping query with nil query text for database: %v", *query.DatabaseName)
			continue
		}
		// Group queries by their database names
		groupMap[*query.DatabaseName] = append(groupMap[*query.DatabaseName], query)
	}

	return groupMap
}

// collectIndividualQueryMetrics collects current query metrics from the performance schema database for the given query IDs
func collectIndividualQueryMetrics(db utils.DataSource, queryIDList []string, queryString string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	// Early exit if queryIDList is empty
	if len(queryIDList) == 0 {
		log.Warn("queryIDList is empty. Skipping further processing as there are no query IDs to process. This might indicate an issue with the query generation or filtering logic.")
		return []utils.IndividualQueryMetrics{}, nil
	}

	// Get the query count threshold
	queryCountThreshold := validator.GetValidQueryCountThreshold(args.QueryMonitoringCountThreshold)

	// Get the query response time threshold
	queryResponseTimeThreshold := validator.GetValidQueryResponseTimeThreshold(args.QueryMonitoringResponseTimeThreshold)

	var metricsList []utils.IndividualQueryMetrics

	for _, queryID := range queryIDList {
		// Combine queryID and thresholds into args
		args := []interface{}{queryID, queryResponseTimeThreshold, min(constants.IndividualQueryCountThreshold, queryCountThreshold)}

		// Use sqlx.In to safely include the slices in the query
		query, args, err := sqlx.In(queryString, args...)
		if err != nil {
			return []utils.IndividualQueryMetrics{}, err
		}

		// Collect the individual query metrics
		metrics, err := utils.CollectMetrics[utils.IndividualQueryMetrics](db, query, args...)
		if err != nil {
			return []utils.IndividualQueryMetrics{}, err
		}

		metricsList = append(metricsList, metrics...)
	}

	return metricsList, nil
}
