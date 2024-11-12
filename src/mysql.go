//go:generate goversioninfo
package main

import (
	"fmt"
	sdk_args "github.com/newrelic/infra-integrations-sdk/v3/args"
	"github.com/newrelic/infra-integrations-sdk/v3/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-mysql/src/query-performance-details"
	"net"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	integrationName = "com.newrelic.mysql"
	nodeEntityType  = "node"
)

type argumentList struct {
	sdk_args.DefaultArgumentList
	Hostname                         string `default:"65.2.189.51" help:"Hostname or IP where MySQL is running."`
	Port                             int    `default:"3306" help:"Port on which MySQL server is listening."`
	Socket                           string `default:"" help:"MySQL Socket file."`
	Username                         string `default:"root" help:"Username for accessing the database."`
	Password                         string `default:"Admin@123" help:"Password for the given user."`
	Database                         string `help:"Database name"`
	ExtraConnectionURLArgs           string `help:"Specify extra connection parameters as attr1=val1&attr2=val2."` // https://github.com/go-sql-driver/mysql#parameters
	InsecureSkipVerify               bool   `default:"false" help:"Skip verification of the server's certificate when using TLS with the connection."`
	EnableTLS                        bool   `default:"false" help:"Use a secure (TLS) connection."`
	RemoteMonitoring                 bool   `default:"false" help:"Identifies the monitored entity as 'remote'. In doubt: set to true"`
	ExtendedMetrics                  bool   `default:"false" help:"Enable extended metrics"`
	ExtendedInnodbMetrics            bool   `default:"false" help:"Enable InnoDB extended metrics"`
	ExtendedMyIsamMetrics            bool   `default:"false" help:"Enable MyISAM extended metrics"`
	OldPasswords                     bool   `default:"false" help:"Allow old passwords: https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords"`
	ShowVersion                      bool   `default:"false" help:"Print build information and exit"`
	EnableQueryPerformanceMonitoring bool   `default:"true" help:"Enable query performance monitoring"`
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
	if args.Socket != "" {
		log.Info("Socket parameter is defined, ignoring host and port parameters")
		return fmt.Sprintf("%s:%s@unix(%s)/%s?%s", args.Username, args.Password, args.Socket, args.Database, query.Encode())
	}

	// Convert hostname and port to DSN address format
	mysqlURL := net.JoinHostPort(args.Hostname, strconv.Itoa(args.Port))

	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", args.Username, args.Password, mysqlURL, args.Database, query.Encode())
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

func main() {
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
	//
	//e, err := createNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	//fatalIfErr(err)
	//
	//db, err := openDB(generateDSN(args))
	//fatalIfErr(err)
	//defer db.close()
	//
	//rawInventory, rawMetrics, err := getRawData(db)
	//fatalIfErr(err)
	//
	//if args.HasInventory() {
	//	populateInventory(e.Inventory, rawInventory)
	//}
	//
	//if args.HasMetrics() {
	//	ms := metricSet(
	//		e,
	//		"MysqlSample",
	//		args.Hostname,
	//		args.Port,
	//		args.RemoteMonitoring,
	//	)
	//	populateMetrics(ms, rawMetrics)
	//}
	fmt.Println("heyyyyasasasas")
	// New functionality
	if args.EnableQueryPerformanceMonitoring {
		fmt.Println("heyyyy")
		query_performance_details.PopulateQueryPerformanceMetrics(args)

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
