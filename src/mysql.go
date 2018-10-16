package main

import (
	"fmt"

	sdk_args "gopkg.in/newrelic/infra-integrations-sdk.v2/args"
	"gopkg.in/newrelic/infra-integrations-sdk.v2/log"
	"gopkg.in/newrelic/infra-integrations-sdk.v2/sdk"
)

const (
	integrationName    = "com.newrelic.mysql"
	integrationVersion = "1.1.0"
)

type argumentList struct {
	sdk_args.DefaultArgumentList
	Hostname              string `default:"localhost" help:"Hostname or IP where MySQL is running."`
	Port                  int    `default:"3306" help:"Port on which MySQL server is listening."`
	Username              string `help:"Username for accessing the database."`
	Password              string `help:"Password for the given user."`
	Database              string `help:"Database name"`
	ExtendedMetrics       bool   `default:"false" help:"Enable extended metrics"`
	ExtendedInnodbMetrics bool   `default:"false" help:"Enable InnoDB extended metrics"`
	ExtendedMyIsamMetrics bool   `default:"false" help:"Enable MyISAM extended metrics"`
}

func generateDSN(args argumentList) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", args.Username, args.Password, args.Hostname, args.Port, args.Database)
}

var args argumentList

func main() {
	integration, err := sdk.NewIntegration(integrationName, integrationVersion, &args)
	fatalIfErr(err)
	log.SetupLogging(args.Verbose)

	sample := integration.NewMetricSet("MysqlSample")

	db, err := openDB(generateDSN(args))
	fatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, err := getRawData(db)
	fatalIfErr(err)

	if args.All || args.Inventory {
		populateInventory(integration.Inventory, rawInventory)
	}

	if args.All || args.Metrics {
		populateMetrics(sample, rawMetrics)
	}

	fatalIfErr(integration.Publish())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
