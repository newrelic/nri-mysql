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
	queryperformancemonitoring "github.com/newrelic/nri-mysql/src/query-performance-monitoring"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

var (
	args      arguments.ArgumentList
	gitCommit = ""
	buildDate = ""
)

func main() {
	i, err := integration.New(constants.IntegrationName, constants.IntegrationVersion, integration.Args(&args))
	utils.FatalIfErr(err)

	if args.ShowVersion {
		fmt.Printf(
			"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
			cases.Title(language.Und).String(strings.Replace(constants.IntegrationName, "com.newrelic.", "", 1)),
			constants.IntegrationVersion,
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			runtime.Version(),
			gitCommit,
			buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	e, err := utils.CreateNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	utils.FatalIfErr(err)

	db, err := openSQLDB(utils.GenerateDSN(args, ""))
	utils.FatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, dbVersion, err := getRawData(db)
	utils.FatalIfErr(err)

	if args.HasInventory() {
		populateInventory(e.Inventory, rawInventory)
	}

	if args.HasMetrics() {
		ms := utils.MetricSet(
			e,
			"MysqlSample",
			args.Hostname,
			args.Port,
			args.RemoteMonitoring,
		)
		populateMetrics(ms, rawMetrics, dbVersion)
	}
	utils.FatalIfErr(i.Publish())

	if args.EnableQueryMonitoring && args.HasMetrics() {
		queryperformancemonitoring.PopulateQueryPerformanceMetrics(args, e, i)
	}
}
