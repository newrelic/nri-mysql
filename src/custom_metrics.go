package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	dbutils "github.com/newrelic/nri-mysql/src/dbutils"
	infrautils "github.com/newrelic/nri-mysql/src/infrautils"
	"gopkg.in/yaml.v3"
)

// openDatabaseConnection opens a new *sql.DB for a given DSN. It is a package-level
// variable so tests can substitute a mock connection opener.
var openDatabaseConnection = func(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}

// CustomMetricsConfig represents YAML configuration structure
type CustomMetricsConfig struct {
	Queries []CustomQuery `yaml:"queries"`
}

// CustomQuery represents a single custom query configuration
type CustomQuery struct {
	Query      string `yaml:"query"`
	SampleName string `yaml:"sample_name"`
	// Database overrides the default database the query runs against.
	// When set, the query runs on a dedicated connection to that database,
	// leaving the shared connection pool's default database untouched.
	Database string `yaml:"database"`
}

// inferMetricType determines New Relic metric type from Go value
func inferMetricType(value interface{}) metric.SourceType {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return metric.GAUGE
	case uint, uint8, uint16, uint32, uint64:
		return metric.GAUGE
	case float32, float64:
		return metric.GAUGE
	default:
		return metric.ATTRIBUTE
	}
}

// convertValue converts []byte values from the MySQL driver into typed Go values.
// The MySQL driver returns all scanned values as []byte when scanning into interface{}.
// This function parses the byte slice into int64, float64, or string as appropriate.
func convertValue(value interface{}) interface{} {
	b, ok := value.([]byte)
	if !ok {
		return value
	}
	s := string(b)
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// processCustomQuery executes a single custom SQL query and collects metrics.
// When database is non-empty, the query runs on a dedicated connection to that
// database instead of the shared pool, so other queries' default database is unaffected.
func processCustomQuery(db *sql.DB, entity *integration.Entity, query string, sampleName string, database string, args arguments.ArgumentList) error {
	if db == nil || entity == nil || query == "" {
		return fmt.Errorf("invalid parameters: db, entity and query are required")
	}

	targetDB := db
	if database != "" {
		altDB, err := openDatabaseConnection(dbutils.GenerateDSN(args, database))
		if err != nil {
			return fmt.Errorf("failed to connect to database %q: %w", database, err)
		}
		defer altDB.Close()
		targetDB = altDB
	}

	rows, err := targetDB.Query(query)
	if err != nil {
		return fmt.Errorf("failed to execute custom query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get column names: %w", err)
	}

	// Create metric set using existing nri-mysql pattern
	ms := infrautils.MetricSet(entity, sampleName, "", 0, false)

	// Process each row
	for rows.Next() {
		// Create slice to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Warn("Failed to scan custom query row: %v", err)
			continue
		}

		// Add metrics to the set
		for i, column := range columns {
			value := convertValue(values[i])
			metricType := inferMetricType(value)

			if err := ms.SetMetric(column, value, metricType); err != nil {
				log.Warn("Failed to set metric %s: %v", column, err)
			}
		}
	}

	return rows.Err()
}

// processCustomConfigFile processes multiple custom queries from YAML configuration
func processCustomConfigFile(db *sql.DB, entity *integration.Entity, configPath string, args arguments.ArgumentList) error {
	if db == nil || entity == nil || configPath == "" {
		return fmt.Errorf("invalid parameters: db, entity and configPath are required")
	}

	// Read YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config CustomMetricsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Process each query
	for i, customQuery := range config.Queries {
		sampleName := customQuery.SampleName
		if sampleName == "" {
			sampleName = "MysqlCustomSample"
		}

		if err := processCustomQuery(db, entity, customQuery.Query, sampleName, customQuery.Database, args); err != nil {
			log.Warn("Failed to process custom query %d: %v", i, err)
			// Continue processing other queries even if one fails
		}
	}

	return nil
}
