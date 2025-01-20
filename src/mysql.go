//go:generate goversioninfo
package main

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/v3/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"

	"os"
	"runtime"
	"strconv"
	"strings"

	arguments "github.com/newrelic/nri-mysql/src/args"
	queryperformancemonitoring "github.com/newrelic/nri-mysql/src/query-performance-monitoring"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

const (
	integrationName = "com.newrelic.mysql"
	nodeEntityType  = "node"
)

var (
	args               arguments.ArgumentList
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func createNodeEntity(
	i *integration.Integration,
	remoteMonitoring bool,
	hostname string,
	port int,
) (*integration.Entity, error) {

	if remoteMonitoring {
		return i.Entity(fmt.Sprint(hostname, ":", port), nodeEntityType)
	}
	return i.LocalEntity(), nil
}

func main() {
	var database string
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	fatalIfErr(err)

	if args.ShowVersion {
		fmt.Printf(
			"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
			strings.Title(strings.Replace(integrationName, "com.newrelic.", "", 1)),
			integrationVersion,
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			runtime.Version(),
			gitCommit,
			buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	e, err := createNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	fatalIfErr(err)

	db, err := openDB(utils.GenerateDSN(args, database))
	fatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, dbVersion, err := getRawData(db)
	fatalIfErr(err)

	if args.HasInventory() {
		populateInventory(e.Inventory, rawInventory)
	}

	if args.HasMetrics() {
		ms := metricSet(
			e,
			"MysqlSample",
			args.Hostname,
			args.Port,
			args.RemoteMonitoring,
		)
		populateMetrics(ms, rawMetrics, dbVersion)
	}
	fatalIfErr(i.Publish())

	if args.EnableQueryMonitoring && args.HasMetrics() {
		queryperformancemonitoring.PopulateQueryPerformanceMetrics(args, e, i)
	}
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
