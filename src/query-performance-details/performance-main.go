package query_performance_details

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

// main
func PopulateQueryPerformanceMetrics(args arguments.ArgumentList) {
	dsn := generateDSN(args)
	db, err := openDB(dsn)
	fatalIfErr(err)
	defer db.close()

	isPreConditionsPassed := validatePreconditions(db)
	if !isPreConditionsPassed {
		fmt.Println("Preconditions failed. Exiting.")
		return
	} else {
		metrics, err := collectQueryMetrics(db)
		if err != nil {
			log.Error("Failed to collect query metrics: %v", err)
			return
		}
		fmt.Println("Metrics collected successfully.", metrics)
		// Data ingestion logic
		// for _, metric := range metrics {
		// 	metric.CollectedAt = time.Now()
		// 	// Add custom event name and other ingestion logic if needed
		// 	log.Info("Collected Query Metric: %+v", metric)
		// }
	}

}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
