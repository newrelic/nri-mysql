package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	sdk_args "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/pkg/errors"
)

const (
	integrationName    = "com.newrelic.mysql"
	integrationVersion = "1.2.0"
	nodeEntityType     = "node"
	pubKeyName         = "serverPubKey"
)

type argumentList struct {
	sdk_args.DefaultArgumentList
	Hostname              string `default:"localhost" help:"Hostname or IP where MySQL is running."`
	Port                  int    `default:"3306" help:"Port on which MySQL server is listening."`
	TLS                   string `default:"false" help:"TLS connection. Values: true, false or skip-verify"`
	ServerPubKey          string `help:"If TLS is set to 'true', the path to the server RSA public key"`
	Username              string `help:"Username for accessing the database."`
	Password              string `help:"Password for the given user."`
	Database              string `help:"Database name"`
	RemoteMonitoring      bool   `default:"false" help:"Identifies the monitored entity as 'remote'. In doubt: set to true"`
	ExtendedMetrics       bool   `default:"false" help:"Enable extended metrics"`
	ExtendedInnodbMetrics bool   `default:"false" help:"Enable InnoDB extended metrics"`
	ExtendedMyIsamMetrics bool   `default:"false" help:"Enable MyISAM extended metrics"`
	OldPasswords          bool   `default:"false" help:"Allow old passwords: https://dev.mysql.com/doc/refman/5.6/en/server-system-variables.html#sysvar_old_passwords"`
}

func loadServerPubkey(path string) error {
	if path == "" {
		return nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return errors.New("failed to decode PEM block containing public key: no key, or not a public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}

	rsaPubKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("not a RSA public key")
	}
	mysql.RegisterServerPubKey(pubKeyName, rsaPubKey)
	return nil
}

func generateDSN(args argumentList) string {
	var params []string
	if args.OldPasswords {
		params = append(params, "allowOldPasswords=true")
	}
	if args.TLS != "" && args.TLS != "false" {
		params = append(params, "tls="+args.TLS)
	}
	if args.ServerPubKey != "" {
		params = append(params, "serverPubKey="+pubKeyName)
	}
	paramsQuery := ""
	if len(params) > 0 {
		paramsQuery = "?" + strings.Join(params, "&")
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s%s", args.Username, args.Password, args.Hostname, args.Port, args.Database, paramsQuery)
}

var args argumentList

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

	log.SetupLogging(args.Verbose)

	e, err := createNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	fatalIfErr(err)

	fatalIfErr(loadServerPubkey(args.ServerPubKey))

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
			metric.Attr("hostname", hostname),
			metric.Attr("port", strconv.Itoa(port)),
		)
	}

	return e.NewMetricSet(
		eventType,
		metric.Attr("port", strconv.Itoa(port)),
	)
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
