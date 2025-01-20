package validator

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// Dynamic error
var (
	ErrPerformanceSchemaDisabled = errors.New("performance schema is not enabled")
	ErrNoRowsFound               = errors.New("no rows found")
	ErrMysqlVersion              = errors.New("only version 8.0+ is supported")
)

// ValidatePreconditions checks if the necessary preconditions are met for performance monitoring.
func ValidatePreconditions(db utils.DataSource) error {
	// Check if Performance Schema is enabled
	performanceSchemaEnabled, errPerformanceEnabled := isPerformanceSchemaEnabled(db)
	if errPerformanceEnabled != nil {
		return errPerformanceEnabled
	}

	if !performanceSchemaEnabled {
		logEnablePerformanceSchemaInstructions(db)
		return ErrPerformanceSchemaDisabled
	}

	// Check if essential consumers are enabled
	errEssentialConsumers := checkEssentialConsumers(db)
	if errEssentialConsumers != nil {
		return fmt.Errorf("essential consumer check failed: %w", errEssentialConsumers)
	}

	// Check if essential instruments are enabled
	errEssentialInstruments := checkEssentialInstruments(db)
	if errEssentialInstruments != nil {
		return fmt.Errorf("essential instruments check failed: %w", errEssentialInstruments)
	}
	return nil
}

// isPerformanceSchemaEnabled checks if the Performance Schema is enabled in the MySQL database.
func isPerformanceSchemaEnabled(db utils.DataSource) (bool, error) {
	var variableName, performanceSchemaEnabled string
	rows, err := db.QueryX("SHOW GLOBAL VARIABLES LIKE 'performance_schema';")
	if err != nil {
		return false, fmt.Errorf("failed to check performance schema status: %w", err)
	}

	if !rows.Next() {
		log.Error("No rows found")
		return false, ErrNoRowsFound
	}

	errScanning := rows.Scan(&variableName, &performanceSchemaEnabled)
	if errScanning != nil {
		return false, errScanning
	}

	return performanceSchemaEnabled == "ON", nil
}

// checkEssentialConsumers checks if the essential consumers are enabled in the Performance Schema.
func checkEssentialConsumers(db utils.DataSource) error {
	// Build the query to check the status of essential consumers
	query := buildConsumerStatusQuery()

	// Execute the query
	rows, err := db.QueryX(query)
	if err != nil {
		return fmt.Errorf("failed to check essential consumers: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Failed to close rows: %v", err)
		}
	}()

	// Check if each essential consumer is enabled
	for rows.Next() {
		var name, enabled string
		if err := rows.Scan(&name, &enabled); err != nil {
			return fmt.Errorf("failed to scan consumer row: %w", err)
		}
		if enabled != "YES" {
			log.Error("Essential consumer %s is not enabled. To enable it, run: UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = '%s';", name, name)
			return fmt.Errorf("%w: %s", utils.ErrEssentialConsumerNotEnabled, name)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("query to check essential consumers failed: %w", err)
	}

	return nil
}

// checkEssentialInstruments checks if the essential instruments are enabled in the Performance Schema.
func checkEssentialInstruments(db utils.DataSource) error {
	// Build the query to check the status of essential instruments
	query := buildInstrumentQuery()

	// Execute the query
	rows, err := db.QueryX(query)
	if err != nil {
		return fmt.Errorf("failed to check essential instruments: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Failed to close rows: %v", err)
		}
	}()

	// Check if each essential instrument is enabled and timed
	for rows.Next() {
		var name, enabled string
		var timed sql.NullString
		if err := rows.Scan(&name, &enabled, &timed); err != nil {
			return fmt.Errorf("failed to scan instrument row: %w", err)
		}
		if enabled != "YES" {
			log.Error("Essential instrument %s is not fully enabled. To enable it, run: UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME = '%s';", name, name)
			return fmt.Errorf("%w: %s", utils.ErrEssentialInstrumentNotEnabled, name)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("query to check essential instruments failed: %w", err)
	}

	return nil
}

// logEnablePerformanceSchemaInstructions logs instructions to enable the Performance Schema.
func logEnablePerformanceSchemaInstructions(db utils.DataSource) {
	version, err := getMySQLVersion(db)
	if err != nil {
		log.Error("Failed to get MySQL version: %v", err)
	}

	if isVersion8OrGreater(version) {
		log.Debug("To enable the Performance Schema, add the following lines to your MySQL configuration file (my.cnf or my.ini) in the [mysqld] section and restart the MySQL server:")
		log.Debug("performance_schema=ON")
	} else {
		log.Error("MySQL version %s is not supported. Only version 8.0+ is supported.", version)
	}
}

// getMySQLVersion retrieves the MySQL version from the database.
func getMySQLVersion(db utils.DataSource) (string, error) {
	query := "SELECT VERSION();"
	rows, err := db.QueryX(query)
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
		return "", utils.ErrMySQLVersion
	}

	return version, nil
}

// isVersion8OrGreater checks if the MySQL version is 8.0 or greater.
func isVersion8OrGreater(version string) bool {
	majorVersion := parseVersion(version)
	return (majorVersion >= 8)
}

// parseVersion extracts the major and minor version numbers from the version string
func parseVersion(version string) int {
	parts := strings.Split(version, ".")
	if len(parts) < constants.MinVersionParts {
		return 0 // Return 0 if the version string is improperly formatted
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Error("Failed to parse major version '%s': %v", parts[0], err)
		return 0
	}

	return majorVersion
}

// buildConsumerStatusQuery constructs a SQL query to check the status of essential consumers
func buildConsumerStatusQuery() string {
	// List of essential consumers to check
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
	query += "'" + strings.Join(consumers, "', '") + "'"
	query += ");"

	return query
}

// buildInstrumentQuery constructs a SQL query to check the status of essential instruments
func buildInstrumentQuery() string {
	// List of essential instruments to check
	instruments := []string{
		"wait/%",
		"statement/%",
		"%lock%",
	}

	// Pre-allocate the slice with the expected length
	instrumentConditions := make([]string, 0, len(instruments))
	for _, instrument := range instruments {
		instrumentConditions = append(instrumentConditions, fmt.Sprintf("NAME LIKE '%s'", instrument))
	}

	// Build the query to check the status of essential instruments
	query := "SELECT NAME, ENABLED, TIMED FROM performance_schema.setup_instruments WHERE "
	query += strings.Join(instrumentConditions, " OR ")
	query += ";"

	return query
}
