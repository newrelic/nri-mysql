package query_performance_details

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
)

func generateDSN(args arguments.ArgumentList) string {
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
	dsn := generateDSN(args)
	db, err := openDB(dsn)
	fatalIfErr(err)
	defer db.close()
	employees, errorPerf := db.queryX("select * from employees")
	fatalIfErr(errorPerf)
	for employees.Next() {
		var emp_no int
		var first_name string
		var birth_date string
		var gender string
		var hire_date string
		var last_name string
		if err := employees.Scan(&emp_no, &first_name, &last_name, &birth_date, &gender, &hire_date); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %d, Name: %s", emp_no, first_name)
	}
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
