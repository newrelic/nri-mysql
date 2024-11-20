package query_performance_details

import (
	"fmt"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/v3/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

// main
func PopulateQueryPerformanceMetrics(args arguments.ArgumentList, e *integration.Entity) {
	dsn := generateDSN(args)
	db, err := openDB(dsn)
	fatalIfErr(err)
	defer db.close()

	isPreConditionsPassed := validatePreconditions(db)
	if !isPreConditionsPassed {
		fmt.Println("Preconditions failed. Exiting.")
		return
	} else {
		rawMetrics, queryIdList, err := collectSlowQueryMetrics(db)
		if err != nil {
			log.Error("Failed to collect query metrics: %v", err)
			return
		}
		fmt.Println("Metrics collected successfully.", rawMetrics)
		rawMetrics1, err1 := collectIndividualQueryDetails(db, queryIdList)
		if err1 != nil {
			log.Error("Failed to collect query metrics: %v", err1)
			return
		}
		fmt.Println("Query details collected successfully.", rawMetrics1)
		rawMetrics2, err2 := captureExecutionPlans(db, rawMetrics1)
		if err2 != nil {
			log.Error("Error populating metrics: %v", err)
			return
		}
		fmt.Println("Query plan details collected successfully.", rawMetrics2)
		// Data ingestion logic for Slow Queries
		// Grouped Slow Queries
		ms := createMetricSet(e, "MysqlSlowQueriesSample", args)
		populateMetrics(ms, rawMetrics)
		// Individual Queries
		populateQueryMetrics(e, args, rawMetrics1)
		// Query Execution Plan Details
		populateQueryPlanMetrics(e, args, rawMetrics2)

		// Start of Wait Event Metrics
		rawMetrics3, err3 := collectWaitEventQueryMetrics(db)
		if err3 != nil {
			log.Error("Failed to collect wait event query metrics: %v", err3)
			return
		}
		fmt.Println("Metrics collected successfully for wait event query metrics.", rawMetrics3)

		// Data ingestion logic for rawMetrics3
		populateWaitEventMetrics(e, args, rawMetrics3)
		// End of Wait Event Metrics

		// Start of Blocking Session Metrics
		rawMetrics4, err4 := collectBlockingSessionMetrics(db)
		if err4 != nil {
			log.Error("Failed to collect blocking session metrics: %v", err4)
			return
		}
		fmt.Println("Metrics collected successfully for blocking session metrics.", rawMetrics4)

		// Data ingestion logic for rawMetrics3
		populateBlockingSessionMetrics(e, args, rawMetrics4)
		// End of Blocking Session Metrics
	}
}

func createMetricSet(e *integration.Entity, sampleName string, args arguments.ArgumentList) *metric.Set {
	return metricSet(
		e,
		sampleName,
		args.Hostname,
		args.Port,
		args.RemoteMonitoring,
	)
}

func metricSet(e *integration.Entity, eventType, hostname string, port int, remoteMonitoring bool) *metric.Set {
	if remoteMonitoring {
		return e.NewMetricSet(
			eventType,
			attribute.Attr("hostname", hostname),
			attribute.Attr("port", strconv.Itoa(port)),
		)
	}

	return e.NewMetricSet(
		eventType,
		attribute.Attr("port", strconv.Itoa(port)),
	)
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
