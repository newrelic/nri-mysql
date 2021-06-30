//go:generate goversioninfo
package main

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"

	sdk_args "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
)

const (
	integrationName = "com.newrelic.mysql"
	nodeEntityType  = "node"
)

type argumentList struct {
	sdk_args.DefaultArgumentList
	Hostname               string `default:"localhost" help:"Hostname or IP where MySQL is running."`
	Port                   int    `default:"3306" help:"Port on which MySQL server is listening."`
	Username               string `help:"Username for accessing the database."`
	Password               string `help:"Password for the given user."`
	Database               string `help:"Database name"`
	ExtraConnectionURLArgs string `help:"Specify extra connection parameters as attr1=val1&attr2=val2."` // https://github.com/go-sql-driver/mysql#parameters
	InsecureSkipVerify     bool   `default:"false" help:"Skip verification of the server's certificate when using TLS with the connection."`
	EnableTLS              bool   `default:"false" help:"Use a secure (TLS) connection."`
	RemoteMonitoring       bool   `default:"false" help:"Identifies the monitored entity as 'remote'. In doubt: set to true"`
	ExtendedMetrics        bool   `default:"false" help:"Enable extended metrics"`
	ExtendedInnodbMetrics  bool   `default:"false" help:"Enable InnoDB extended metrics"`
	ExtendedMyIsamMetrics  bool   `default:"false" help:"Enable MyISAM extended metrics"`
	OldPasswords           bool   `default:"false" help:"Allow old passwords: https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords"`
	ShowVersion            bool   `default:"false" help:"Print build information and exit"`
}

func generateDSN(args argumentList) string {
	// Format query parameters
	query := url.Values{}
	if args.OldPasswords {
		query.Add("allowOldPasswords", "true")
	}
	if args.EnableTLS {
		query.Add("tls", "true")
	}
	if args.InsecureSkipVerify {
		query.Add("tls", "skip-verify")
	}
	extraArgsMap, err := url.ParseQuery(args.ExtraConnectionURLArgs)
	if err == nil {
		for k, v := range extraArgsMap {
			query.Add(k, v[0])
		}
	} else {
		log.Warn("Could not successfully parse ExtraConnectionURLArgs.", err.Error())
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", args.Username, args.Password, args.Hostname, args.Port, args.Database, query.Encode())
}

var (
	args               argumentList
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

func createIntegration() (*integration.Integration, error) {
	cachePath := os.Getenv("NRIA_CACHE_PATH")
	if cachePath == "" {
		return integration.New(integrationName, integrationVersion, integration.Args(&args))
	}

	l := log.NewStdErr(args.Verbose)
	s, err := persist.NewFileStore(cachePath, l, persist.DefaultTTL)
	if err != nil {
		return nil, err
	}

	return integration.New(
		integrationName,
		integrationVersion,
		integration.Args(&args),
		integration.Storer(s),
		integration.Logger(l),
	)

}

func main() {

	i, err := createIntegration()
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

	db, err := openDB(generateDSN(args))
	fatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, err := getRawData(db)
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
		populateMetrics(ms, rawMetrics)
	}

	fatalIfErr(i.Publish())
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
