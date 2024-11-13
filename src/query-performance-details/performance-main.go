package query_performance_details

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	"net"
	"net/url"
	"strconv"
)

func generateDSN(args arguments.ArgumentList) string {
	fmt.Println("arg12wqswds: %v", args)
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

func PopulateQueryPerformanceMetrics(args arguments.ArgumentList) {
	fmt.Println("args: %v", args)
	fmt.Println("argList: %v %v", args)
	dsn := generateDSN(args)
	fmt.Println("dsn: %v", dsn)
	fmt.Println()
	db, err := openDB(dsn)
	fatalIfErr(err)
	defer db.close()
	inventory, errorPerf := db.query("SHOW GLOBAL VARIABLES")
	fatalIfErr(errorPerf)
	fmt.Println("Populaing query %v", inventory)
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
