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

/*
NOTE: This procedure (`newrelic.enable_essential_consumers_and_instruments`) enables essential consumers
and instruments for MySQL query performance monitoring. It's not part of default MySQL and must be created
during initial setup for self-hosted, RDS, or Aurora MySQL servers.

For detailed setup instructions, see: https://docs.newrelic.com/install/mysql
*/
const enableEssentialConsumersAndInstrumentsProcedureQuery = "CALL newrelic.enable_essential_consumers_and_instruments();"

/*
SQL template for updating the status of essential consumers in the Performance Schema.
This template provides the SQL command to enable a specific consumer by updating its status in the setup_consumers table.
*/
const essentialConsumerNotEnabledWarning = "Essential consumer %s is not enabled. To enable it, run: UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = '%s';"

// EP (Explicit Queries): Execute explicit SQL queries to enable essential consumers and instruments.
var QueriesToEnableEssentialConsumersAndInstruments = []string{
	"UPDATE performance_schema.setup_consumers SET enabled='YES' WHERE name LIKE 'events_statements_%' OR name LIKE 'events_waits_%';",
	"UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'wait/%' OR NAME LIKE 'statement/%' OR NAME LIKE '%lock%';",
}

// Dynamic error
var (
	ErrImproperlyFormattedVersion = errors.New("version string is improperly formatted")
	ErrPerformanceSchemaDisabled  = errors.New("performance schema is not enabled")
	ErrNoRowsFound                = errors.New("no rows found")
	ErrMySQLVersion               = errors.New("only version 8.0+ is supported")
	ErrUnsupportedMySQLVersion    = errors.New("MySQL version is not supported")
)

// ConsumerStatus represents the status of a consumer in the Performance Schema.
type ConsumerStatus struct {
	Name    string `db:"NAME"`
	Enabled string `db:"ENABLED"`
}

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
	errEssentialConsumers := checkAndEnableEssentialConsumers(db)
	if errEssentialConsumers != nil {
		log.Warn("Essential consumer check failed: %v", errEssentialConsumers)
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

// numberOfEssentialConsumersEnabled executes a query to check if essential items are enabled.
func numberOfEssentialConsumersEnabled(db utils.DataSource, query string) (count int, essentialError error) {
	// Use CollectMetrics to get the consumer statuses
	consumerStatuses, err := utils.CollectMetrics[ConsumerStatus](db, query)
	if err != nil {
		return 0, fmt.Errorf("failed to check essential status: %w", err)
	}

	enabledCount := 0
	for _, status := range consumerStatuses {
		if strings.ToUpper(status.Enabled) == "YES" {
			enabledCount++
		} else {
			log.Warn(essentialConsumerNotEnabledWarning, status.Name, status.Name)
		}
	}

	return enabledCount, nil
}

/*
Enables essential Performance Schema consumers and instruments using either a stored procedure
or direct SQL queries. First attempts to use the custom procedure 'newrelic.enable_essential_consumers_and_instruments',
then falls back to explicit queries if needed.

Used primarily for AWS RDS instances and self-hosted MySQL servers. For setup details, see:
https://docs.newrelic.com/install/mysql

Parameters:
- db: The database connection object.

Returns:
- An error if both methods fail.
*/
func enableEssentialConsumersAndInstruments(db utils.DataSource) error {
	log.Debug("Attempting to enable essential consumers and instruments via stored procedure...")
	err := enableViaStoredProcedure(db)
	if err == nil {
		log.Debug("Successfully enabled essential consumers and instruments via stored procedure")
		return nil
	}

	// Check if error is related to stored procedure not existing or permissions
	// These are errors where falling back to explicit queries might help
	errMsg := strings.ToLower(err.Error())
	if isRecoverableError(errMsg) {
		log.Debug("Stored procedure failed with recoverable error, attempting fallback to explicit queries: %v", err)
		return enableViaExplicitQueries(db)
	}

	// For other errors (like connection issues), don't attempt fallback
	return fmt.Errorf("failed to enable essential consumers and instruments: %w", err)
}

func isRecoverableError(errMsg string) bool {
	recoverablePatterns := []string{
		"procedure newrelic.enable_essential_consumers_and_instruments does not exist",
		"routine newrelic.enable_essential_consumers_and_instruments does not exist",
		"permission denied",
	}

	for _, pattern := range recoverablePatterns {
		if strings.Contains(errMsg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func enableViaStoredProcedure(db utils.DataSource) error {
	_, err := db.QueryX(enableEssentialConsumersAndInstrumentsProcedureQuery)
	if err != nil {
		return fmt.Errorf("failed to execute stored procedure to enable essential consumers and instruments: %w", err)
	}
	return nil
}

func enableViaExplicitQueries(db utils.DataSource) error {
	log.Debug("Attempting to enable essential consumers and instruments via explicit queries...")
	for _, query := range QueriesToEnableEssentialConsumersAndInstruments {
		_, err := db.QueryX(query)
		if err != nil {
			log.Error("Failed to execute query '%s': %v", query, err)
			return fmt.Errorf("failed to execute query '%s': %w", query, err)
		}
	}

	log.Debug("Successfully enabled essential consumers and instruments via explicit queries")
	return nil
}

// checkAndEnableEssentialConsumers checks if the essential consumers are enabled in the Performance Schema.
// If fewer than the required number of consumers are enabled, it attempts to enable them
// via the newrelic.enable_essential_consumers_and_instruments stored procedure.
func checkAndEnableEssentialConsumers(db utils.DataSource) error {
	query := buildConsumerStatusQuery()
	count, consumerErr := numberOfEssentialConsumersEnabled(db, query)

	// If there was an error checking consumers, return it immediately
	if consumerErr != nil {
		return consumerErr
	}

	// If the count of enabled essential consumers is less than the required count, try to enable them
	if count < constants.EssentialConsumersCount {
		if err := enableEssentialConsumersAndInstruments(db); err != nil {
			return fmt.Errorf("failed to enable essential consumers and instruments: %w", err)
		}
	}

	// If we've made it here, everything is successful
	return nil
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
