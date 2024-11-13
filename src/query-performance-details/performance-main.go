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

	performanceSchemaEnabled, err := isPerformanceSchemaEnabled(db)
	if !performanceSchemaEnabled {
		fmt.Println("Performance Schema is not enabled. Skipping validation.")
		return
	}

	errEssentialConsumer := checkEssentialConsumers(db)
	if errEssentialConsumer != nil {
		fmt.Printf("Essential consumer check failed\n")
		return
	}

}

func isPerformanceSchemaEnabled(db dataSource) (bool, error) {
	var variableName, performanceSchemaEnabled string
	rows, err := db.queryX("SHOW GLOBAL VARIABLES LIKE 'performance_schema';")

	if !rows.Next() {
		fmt.Println("No rows found")
		return false, nil
	}

	if errScanning := rows.Scan(&variableName, &performanceSchemaEnabled); err != nil {
		fatalIfErr(errScanning)
	}

	if err != nil {
		return false, fmt.Errorf("failed to check Performance Schema status: %w", err)
	}
	return performanceSchemaEnabled == "ON", nil
}

func checkEssentialConsumers(db dataSource) error {
	consumers := []string{
		"events_waits_current",
		"events_waits_history_long",
		"events_waits_history",
		"events_statements_history_long",
		"events_statements_history",
		"events_statements_current",
		"events_statements_cpu",
		"events_transactions_current",
		"events_stages_current",
	}

	query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ("
	for i, consumer := range consumers {
		if i > 0 {
			query += ", "
		}
		query += fmt.Sprintf("'%s'", consumer)
	}
	query += ");"

	rows, err := db.queryX(query)
	if err != nil {
		return fmt.Errorf("failed to check essential consumers: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var name, enabled string
		if err := rows.Scan(&name, &enabled); err != nil {
			return fmt.Errorf("failed to scan consumer row: %w", err)
		}
		if enabled != "YES" {
			log.Error("Essential consumer %s is not enabled. To enable it, run: UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = '%s';", name, name)
			return fmt.Errorf("essential consumer %s is not enabled", name)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}

	return nil
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
