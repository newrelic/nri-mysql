package query_performance_details

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

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

	if isVersion8OrGreater(version) {
		log.Info("To enable the Performance Schema, add the following lines to your MySQL configuration file (my.cnf or my.ini) in the [mysqld] section and restart the MySQL server:")
		fmt.Println("To enable the Performance Schema, add the following lines to your MySQL configuration file (my.cnf or my.ini) in the [mysqld] section and restart the MySQL server:")
		log.Info("performance_schema=ON")
		fmt.Println("performance_schema=ON")

		log.Info("For MySQL 8.0 and higher, you may also need to set the following variables:")
		fmt.Println("For MySQL 8.0 and higher, you may also need to set the following variables:")
		log.Info("performance_schema_instrument='%%=ON'")
		fmt.Println("performance_schema_instrument='%=ON'")
		log.Info("performance_schema_consumer_events_statements_current=ON")
		fmt.Println("performance_schema_consumer_events_statements_current=ON")
		log.Info("performance_schema_consumer_events_statements_history=ON")
		fmt.Println("performance_schema_consumer_events_statements_history=ON")
		log.Info("performance_schema_consumer_events_statements_history_long=ON")
		fmt.Println("performance_schema_consumer_events_statements_history_long=ON")
		log.Info("performance_schema_consumer_events_waits_current=ON")
		fmt.Println("performance_schema_consumer_events_waits_current=ON")
		log.Info("performance_schema_consumer_events_waits_history=ON")
		fmt.Println("performance_schema_consumer_events_waits_history=ON")
		log.Info("performance_schema_consumer_events_waits_history_long=ON")
		fmt.Println("performance_schema_consumer_events_waits_history_long=ON")
	} else {
		log.Error("MySQL version %s is not supported. Only version 8.0+ is supported.", version)
		fmt.Printf("MySQL version %s is not supported. Only version 8.0+ is supported.\n", version)
	}

}

func getMySQLVersion(db dataSource) (string, error) {
	query := "SELECT VERSION();"
	rows, err := db.queryX(query)
	if err != nil {
		return "", fmt.Errorf("failed to execute version query: %w", err)
	}
	defer rows.Close()

	var version string
	if rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return "", fmt.Errorf("failed to scan version: %w", err)
		}
	}

	if version == "" {
		return "", fmt.Errorf("failed to determine MySQL version")
	}

	return version, nil
}

func isVersion8OrGreater(version string) bool {
	majorVersion, minorVersion := parseVersion(version)
	return (majorVersion > 8) || (majorVersion == 8 && minorVersion >= 0)
}

// parseVersion extracts the major and minor version numbers from the version string
func parseVersion(version string) (int, int) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0 // Return 0 if the version string is improperly formatted
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Error("Failed to parse major version: %v", err)
		return 0, 0
	}

	minorVersion, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Error("Failed to parse minor version: %v", err)
		return 0, 0
	}

	return majorVersion, minorVersion
}
