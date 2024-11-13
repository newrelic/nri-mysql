package query_performance_details

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

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

// main
func PopulateQueryPerformanceMetrics(args arguments.ArgumentList) {
	dsn := generateDSN(args)
	db, err := openDB(dsn)
	fatalIfErr(err)
	defer db.close()

	isPreConditionsPassed := validatePreconditions(db)
	if !isPreConditionsPassed {
		fmt.Println("Preconditions failed. Exiting.")
		return
	}

}

func validatePreconditions(db dataSource) bool {

	performanceSchemaEnabled, errPerformanceEnabled := isPerformanceSchemaEnabled(db)
	if errPerformanceEnabled != nil {
		log.Error("Failed to check Performance Schema status: %v", errPerformanceEnabled)
		return false
	}

	if !performanceSchemaEnabled {
		log.Error("Performance Schema is not enabled. Skipping validation.")
		logEnablePerformanceSchemaInstructions(db)
		return false
	}

	errEssentialConsumers := checkEssentialConsumers(db)
	if errEssentialConsumers != nil {
		log.Error("Essential consumer check failed\n")
		return false
	}

	errEssentialInstruments := checkEssentialInstruments(db)
	if errEssentialInstruments != nil {
		log.Error("Essential instruments check failed\n")
		return false
	}
	return true
}

func isPerformanceSchemaEnabled(db dataSource) (bool, error) {
	var variableName, performanceSchemaEnabled string
	rows, err := db.queryX("SHOW GLOBAL VARIABLES LIKE 'performance_schema';")

	if !rows.Next() {
		log.Error("No rows found")
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

func checkEssentialInstruments(db dataSource) error {
	instruments := []string{
		// Add other essential instruments here
		"wait/%",
		"statement/%",
		"%lock%",
	}

	var instrumentConditions []string
	for _, instrument := range instruments {
		instrumentConditions = append(instrumentConditions, fmt.Sprintf("NAME LIKE '%s'", instrument))
	}

	query := "SELECT NAME, ENABLED, TIMED FROM performance_schema.setup_instruments WHERE "
	query += strings.Join(instrumentConditions, " OR ")
	query += ";"

	rows, err := db.queryX(query)
	if err != nil {
		return fmt.Errorf("failed to check essential instruments: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var name, enabled string
		var timed sql.NullString
		if err := rows.Scan(&name, &enabled, &timed); err != nil {
			return fmt.Errorf("failed to scan instrument row: %w", err)
		}
		if enabled != "YES" || (timed.Valid && timed.String != "YES") {
			log.Error("Essential instrument %s is not fully enabled. To enable it, run: UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME = '%s';", name, name)
			return fmt.Errorf("essential instrument %s is not fully enabled", name)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}

	return nil
}

func logEnablePerformanceSchemaInstructions(db dataSource) {
	version, err := getMySQLVersion(db)
	if err != nil {
		log.Error("Failed to get MySQL version: %v", err)
		return
	}

	log.Error("To enable the Performance Schema, add the following line to your MySQL configuration file (my.cnf or my.ini) and restart the MySQL server:")
	log.Error("performance_schema=ON")

	if strings.HasPrefix(version, "5.6") {
		log.Error("For MySQL 5.6, you may also need to set the following variables:")
		log.Error("performance_schema_instrument='%=ON'")
		log.Error("performance_schema_consumer_events_statements_current=ON")
		log.Error("performance_schema_consumer_events_statements_history=ON")
		log.Error("performance_schema_consumer_events_statements_history_long=ON")
		log.Error("performance_schema_consumer_events_waits_current=ON")
		log.Error("performance_schema_consumer_events_waits_history=ON")
		log.Error("performance_schema_consumer_events_waits_history_long=ON")
	} else if strings.HasPrefix(version, "5.7") || strings.HasPrefix(version, "8.0") {
		log.Error("For MySQL 5.7 and 8.0, you may also need to set the following variables:")
		log.Error("performance_schema_instrument='%=ON'")
		log.Error("performance_schema_consumer_events_statements_current=ON")
		log.Error("performance_schema_consumer_events_statements_history=ON")
		log.Error("performance_schema_consumer_events_statements_history_long=ON")
		log.Error("performance_schema_consumer_events_waits_current=ON")
		log.Error("performance_schema_consumer_events_waits_history=ON")
		log.Error("performance_schema_consumer_events_waits_history_long=ON")
	}
}

func getMySQLVersion(db dataSource) (string, error) {
	var version string
	row, err := db.queryX("SELECT VERSION();")
	if row.Next() {
		err = row.Scan(&version)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get MySQL version: %w", err)
	}
	return version, nil
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
