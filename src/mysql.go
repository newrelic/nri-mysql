package main

import (
	"fmt"

	sdk_args "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
)

const (
	integrationName    = "com.newrelic.mysql"
	integrationVersion = "1.2.0"
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
	OldPasswords          bool   `default:"false" help:"Allow old passwords: https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords"`
}

func generateDSN(args argumentList) string {
	params := ""
	if args.OldPasswords {
		params = "?allowOldPasswords=true"
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s%s", args.Username, args.Password, args.Hostname, args.Port, args.Database, params)
}

var args argumentList

func main() {
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	fatalIfErr(err)
	log.SetupLogging(args.Verbose)

	e := i.LocalEntity()

	db, err := openDB(generateDSN(args))
	fatalIfErr(err)
	defer db.close()

	rawInventory, rawMetrics, err := getRawData(db)
	fatalIfErr(err)

	if args.HasInventory() {
		populateInventory(e.Inventory, rawInventory)
	}

	if args.HasMetrics() {
		ms := e.NewMetricSet("MysqlSample", metric.Attr("hostname", args.Hostname))
		populateMetrics(ms, rawMetrics)
	}

	fatalIfErr(i.Publish())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
