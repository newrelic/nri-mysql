//go:generate goversioninfo
package main

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"

	"os"
	"runtime"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	arguments "github.com/newrelic/nri-mysql/src/args"
	dbutils "github.com/newrelic/nri-mysql/src/dbutils"
	infrautils "github.com/newrelic/nri-mysql/src/infrautils"
	queryperformancemonitoring "github.com/newrelic/nri-mysql/src/query-performance-monitoring"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
)

var (
	args               arguments.ArgumentList
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func main() {
	i, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	if args.ShowVersion {
		fmt.Printf(
			"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
			cases.Title(language.Und).String(strings.Replace(constants.IntegrationName, "com.newrelic.", "", 1)),
			integrationVersion,
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			runtime.Version(),
			gitCommit,
			buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	e, err := infrautils.CreateNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	infrautils.FatalIfErr(err)

	/*
		If the QueryMonitoringOnly flag is set, only populate query performance metrics
		and return from main function without proceeding further.
	*/
	if args.QueryMonitoringOnly {
		queryperformancemonitoring.PopulateQueryPerformanceMetrics(args, e, i)
		return
	}

	db, err := openSQLDB(dbutils.GenerateDSN(args, ""))
	infrautils.FatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, dbVersion, err := getRawData(db)
	infrautils.FatalIfErr(err)

	if args.HasInventory() {
		populateInventory(e.Inventory, rawInventory)
	}

	if args.HasMetrics() {
		ms := infrautils.MetricSet(
			e,
			"MysqlSample",
			args.Hostname,
			args.Port,
			args.RemoteMonitoring,
		)
		populateMetrics(ms, rawMetrics, dbVersion)
	}
	infrautils.FatalIfErr(i.Publish())

	if args.EnableQueryMonitoring {
		queryperformancemonitoring.PopulateQueryPerformanceMetrics(args, e, i)
	}
}
