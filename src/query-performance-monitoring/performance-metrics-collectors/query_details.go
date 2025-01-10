package performancemetricscollectors

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// PopulateSlowQueryMetrics collects and sets slow query metrics and returns the list of query IDs
func PopulateSlowQueryMetrics(i *integration.Integration, e *integration.Entity, db utils.DataSource, args arguments.ArgumentList, excludedDatabases []string) []string {
	rawMetrics, queryIDList, err := collectGroupedSlowQueryMetrics(db, args.SlowQueryFetchInterval, args.QueryCountThreshold, excludedDatabases)
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
	query, args, err := sqlx.In(utils.SlowQueries, slowQueryfetchInterval, excludedDatabases, min(queryCountThreshold, constants.MaxQueryCountThreshold))
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
			log.Warn("Query ID is nil")
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
func PopulateIndividualQueryDetails(db utils.DataSource, queryIDList []string, i *integration.Integration, e *integration.Entity, args arguments.ArgumentList) []utils.QueryGroup {
	currentQueryMetrics, currentQueryMetricsErr := currentQueryMetrics(db, queryIDList, args)
	if currentQueryMetricsErr != nil {
		log.Error("Failed to collect current query metrics: %v", currentQueryMetricsErr)
		return nil
	}

	recentQueryList, recentQueryErr := recentQueryMetrics(db, queryIDList, args)
	if recentQueryErr != nil {
		log.Error("Failed to collect recent query metrics: %v", recentQueryErr)
		return nil
	}

	extensiveQueryList, extensiveQueryErr := extensiveQueryMetrics(db, queryIDList, args)
	if extensiveQueryErr != nil {
		log.Error("Failed to collect history query metrics: %v", extensiveQueryErr)
		return nil
	}

	queryList := append(append(currentQueryMetrics, recentQueryList...), extensiveQueryList...)
	newMetricsList := make([]utils.IndividualQueryMetrics, len(queryList))
	copy(newMetricsList, queryList)
	metricList := make([]interface{}, 0, len(newMetricsList))
	for i := range newMetricsList {
		newMetricsList[i].QueryText = nil
		metricList = append(metricList, newMetricsList[i])
	}

	err := utils.IngestMetric(metricList, "MysqlIndividualQueriesSample", i, args)
	if err != nil {
		log.Error("Failed to ingest individual query metrics: %v", err)
		return nil
	}
	groupQueriesByDatabase := groupQueriesByDatabase(queryList)

	return groupQueriesByDatabase
}

// groupQueriesByDatabase groups queries by their database name
func groupQueriesByDatabase(filteredList []utils.IndividualQueryMetrics) []utils.QueryGroup {
	groupMap := make(map[string][]utils.IndividualQueryMetrics)

	for _, query := range filteredList {
		if query.DatabaseName == nil {
			log.Warn("Database name is nil")
			continue
		}
		groupMap[*query.DatabaseName] = append(groupMap[*query.DatabaseName], query)
	}

	// Pre-allocate the slice with the length of the groupMap
	groupedQueries := make([]utils.QueryGroup, 0, len(groupMap))
	for dbName, queries := range groupMap {
		groupedQueries = append(groupedQueries, utils.QueryGroup{
			Database: dbName,
			Queries:  queries,
		})
	}

	return groupedQueries
}

// currentQueryMetrics collects current query metrics from the performance schema database for the given query IDs
func currentQueryMetrics(db utils.DataSource, queryIDList []string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	metrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.CurrentRunningQueriesSearch, args)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// recentQueryMetrics collects recent query metrics	from the performance schema	database for the given query IDs
func recentQueryMetrics(db utils.DataSource, queryIDList []string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	metrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.RecentQueriesSearch, args)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// extensiveQueryMetrics collects extensive query metrics from the performance schema database for the given query IDs
func extensiveQueryMetrics(db utils.DataSource, queryIDList []string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	metrics, err := collectIndividualQueryMetrics(db, queryIDList, utils.PastQueriesSearch, args)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// collectIndividualQueryMetrics collects current query metrics from the performance schema database for the given query IDs
func collectIndividualQueryMetrics(db utils.DataSource, queryIDList []string, queryString string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
	// Early exit if queryIDList is empty
	if len(queryIDList) == 0 {
		log.Warn("queryIDList is empty")
		return []utils.IndividualQueryMetrics{}, nil
	}

	var metricsList []utils.IndividualQueryMetrics

	for _, queryID := range queryIDList {
		// Combine queryID and thresholds into args
		args := []interface{}{queryID, args.QueryResponseTimeThreshold, min(constants.IndividualQueryCountThreshold, args.QueryCountThreshold)}

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
