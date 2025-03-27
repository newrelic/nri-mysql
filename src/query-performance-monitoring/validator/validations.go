package validator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
)

// Query to check if the Performance Schema is enabled
const performanceSchemaQuery = "SHOW GLOBAL VARIABLES LIKE 'performance_schema';"

// Query to get the MySQL version
const versionQuery = "SELECT VERSION();"

// Dynamic error
var (
	ErrImproperlyFormattedVersion = errors.New("version string is improperly formatted")
	ErrPerformanceSchemaDisabled  = errors.New("performance schema is not enabled")
	ErrNoRowsFound                = errors.New("no rows found")
	ErrMysqlVersion               = errors.New("only version 8.0+ is supported")
	ErrUnsupportedMySQLVersion    = errors.New("MySQL version is not supported")
)

// ValidatePreconditions checks if the necessary preconditions are met for performance monitoring.
func ValidatePreconditions(db utils.DataSource) error {
	// Get the MySQL version
	version, err := getMySQLVersion(db)
	if err != nil {
		log.Error("Failed to get MySQL version: %v", err)
		return err
	}

	// Check if the MySQL version is supported
	if !isVersion8OrGreater(version) {
		log.Error("MySQL version %s is not supported. Only version 8.0+ is supported.", version)
		return fmt.Errorf("%w: MySQL version %s is not supported. Only version 8.0+ is supported", ErrUnsupportedMySQLVersion, version)
	}

	// Check if Performance Schema is enabled
	performanceSchemaEnabled, errPerformanceEnabled := isPerformanceSchemaEnabled(db)
	if errPerformanceEnabled != nil {
		return errPerformanceEnabled
	}

	if !performanceSchemaEnabled {
		logEnablePerformanceSchemaInstructions(version)
		return ErrPerformanceSchemaDisabled
	}

	// Check if essential consumers are enabled
	errEssentialConsumers := checkEssentialConsumers(db)
	if errEssentialConsumers != nil {
		log.Warn("Essential consumer check failed: %v", errEssentialConsumers)
	}

	// Check if essential instruments are enabled
	errEssentialInstruments := checkEssentialInstruments(db)
	if errEssentialInstruments != nil {
		log.Warn("Essential instruments check failed: %v", errEssentialInstruments)
	}
	return nil
}

// isPerformanceSchemaEnabled checks if the Performance Schema is enabled in the MySQL database.
func isPerformanceSchemaEnabled(db utils.DataSource) (bool, error) {
	var variableName, performanceSchemaEnabled string
	rows, err := db.QueryX(performanceSchemaQuery)
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

// checkEssentialStatus executes a query to check if essential items are enabled.
func checkEssentialStatus(db utils.DataSource, query string, updateSQLTemplate string, errMsgTemplate error, consumerCheck bool) (count int, essentialError error) {
	enabledCount := 0

	// Execute the query
	rows, err := db.QueryX(query)
	if err != nil {
		return enabledCount, fmt.Errorf("failed to check essential status: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Failed to close rows: %v", err)
		}
	}()

	for rows.Next() {
		var name, enabled string
		if err := rows.Scan(&name, &enabled); err != nil {
			return enabledCount, fmt.Errorf("failed to scan row: %w", err)
		}
		if consumerCheck && enabled == "YES" {
			enabledCount++
		}
		if !consumerCheck && enabled != "YES" {
			log.Warn(updateSQLTemplate, name, name)
			return enabledCount, fmt.Errorf("%w: %s", errMsgTemplate, name)
		}
	}

	if err := rows.Err(); err != nil {
		return enabledCount, fmt.Errorf("query to check essential status failed: %w", err)
	}

	return enabledCount, nil
}

// callEnableConsumersProcedure calls a stored procedure to enable the consumers.
func callEnableConsumersProcedure(db utils.DataSource) error {
	_, err := db.QueryX("CALL newrelic.enable_events_statements_consumers()")
	if err != nil {
		return fmt.Errorf("failed to execute stored procedure: %w", err)
	}
	return nil
}

// checkEssentialConsumers checks if the essential consumers are enabled in the Performance Schema.
func checkEssentialConsumers(db utils.DataSource) error {
	query := buildConsumerStatusQuery()
	updateSQLTemplate := "Essential consumer %s is not enabled. To enable it, run: UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = '%s';"
	count, consumerErr := checkEssentialStatus(db, query, updateSQLTemplate, utils.ErrEssentialConsumerNotEnabled, true)
	// If the count of enabled items is less than 3, call the stored procedure
	if count < constants.EssentialConsumersCount {
		if err := callEnableConsumersProcedure(db); err != nil {
			return fmt.Errorf("failed to call stored procedure to enable consumers: %w", err)
		}
	}
	return consumerErr
}

// checkEssentialInstruments checks if the essential instruments are enabled in the Performance Schema.
func checkEssentialInstruments(db utils.DataSource) error {
	query := buildInstrumentQuery()
	updateSQLTemplate := "Essential instrument %s is not fully enabled. To enable it, run: UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME = '%s';"
	_, instrumentsErr := checkEssentialStatus(db, query, updateSQLTemplate, utils.ErrEssentialInstrumentNotEnabled, false)
	return instrumentsErr
}

// logEnablePerformanceSchemaInstructions logs instructions to enable the Performance Schema.
func logEnablePerformanceSchemaInstructions(version string) {
	if isVersion8OrGreater(version) {
		log.Debug("To enable the Performance Schema, add the following lines to your MySQL configuration file (my.cnf or my.ini) in the [mysqld] section and restart the MySQL server:")
		log.Debug("performance_schema=ON")
	} else {
		log.Error("MySQL version %s is not supported. Only version 8.0+ is supported.", version)
	}
}

// getMySQLVersion retrieves the MySQL version from the database.
func getMySQLVersion(db utils.DataSource) (string, error) {
	rows, err := db.QueryX(versionQuery)
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
	majorVersion, err := extractMajorFromVersion(version)
	if err != nil {
		log.Error("Failed to extract major version: %v", err)
		return false
	}
	return (majorVersion >= 8)
}

// extractMajorFromVersion extracts the major version number from a version string.
func extractMajorFromVersion(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < constants.MinVersionParts {
		return 0, ErrImproperlyFormattedVersion
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Error("Failed to parse major version '%s': %v", parts[0], err)
		return 0, err
	}

	return majorVersion, nil
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
	query := "SELECT NAME, ENABLED FROM performance_schema.setup_instruments WHERE "
	query += strings.Join(instrumentConditions, " OR ")
	query += ";"

	return query
}

// GetValidSlowQueryFetchIntervalThreshold validates and returns the appropriate value
func GetValidSlowQueryFetchIntervalThreshold(threshold int) int {
	if threshold < 0 {
		log.Warn("Slow query fetch interval threshold is negative, setting to default value: %d", constants.DefaultSlowQueryFetchInterval)
		return constants.DefaultSlowQueryFetchInterval
	}
	return threshold
}

// getValidQueryResponseTimeThreshold validates and returns the appropriate value
func GetValidQueryResponseTimeThreshold(threshold int) int {
	if threshold < 0 {
		log.Warn("Query response time threshold is negative, setting to default value: %d", constants.DefaultQueryResponseTimeThreshold)
		return constants.DefaultQueryResponseTimeThreshold
	}
	return threshold
}

// getValidQueryCountThreshold validates and returns the appropriate value
func GetValidQueryCountThreshold(threshold int) int {
	if threshold < 0 {
		log.Warn("Query count threshold is negative, setting to default value: %d", constants.DefaultQueryCountThreshold)
		return constants.DefaultQueryCountThreshold
	} else if threshold >= constants.MaxQueryCountThreshold {
		log.Warn("Query count threshold is greater than max supported value, setting to max supported value: %d", constants.MaxQueryCountThreshold)
		return constants.MaxQueryCountThreshold
	}
	return threshold
}
