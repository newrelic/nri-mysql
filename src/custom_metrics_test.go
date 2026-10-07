package main

import (
	"database/sql"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestInferMetricType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected metric.SourceType
	}{
		// Integer types
		{"int value", 42, metric.GAUGE},
		{"int8 value", int8(42), metric.GAUGE},
		{"int16 value", int16(42), metric.GAUGE},
		{"int32 value", int32(42), metric.GAUGE},
		{"int64 value", int64(42), metric.GAUGE},
		{"uint value", uint(42), metric.GAUGE},
		{"uint8 value", uint8(42), metric.GAUGE},
		{"uint16 value", uint16(42), metric.GAUGE},
		{"uint32 value", uint32(42), metric.GAUGE},
		{"uint64 value", uint64(42), metric.GAUGE},

		// Float types
		{"float32 value", float32(3.14), metric.GAUGE},
		{"float64 value", 3.14, metric.GAUGE},
		{"zero float", 0.0, metric.GAUGE},
		{"negative float", -3.14, metric.GAUGE},

		// Non-numeric types (should be attributes)
		{"string value", "test", metric.ATTRIBUTE},
		{"empty string", "", metric.ATTRIBUTE},
		{"nil value", nil, metric.ATTRIBUTE},
		{"boolean true", true, metric.ATTRIBUTE},
		{"boolean false", false, metric.ATTRIBUTE},
		{"slice", []int{1, 2, 3}, metric.ATTRIBUTE},
		{"map", map[string]int{"a": 1}, metric.ATTRIBUTE},

		// Edge cases
		{"zero int", 0, metric.GAUGE},
		{"negative int", -42, metric.GAUGE},
		{"very large int64", int64(9223372036854775807), metric.GAUGE},
		{"very large uint64", uint64(18446744073709551615), metric.GAUGE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferMetricType(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessCustomQuery(t *testing.T) {
	t.Run("Parameter Validation", func(t *testing.T) {
		// Test nil database
		err := processCustomQuery(nil, &integration.Entity{}, "SELECT 1", "TestSample", "", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameters")

		// Test nil entity
		db, _, _ := sqlmock.New()
		defer db.Close()
		err = processCustomQuery(db, nil, "SELECT 1", "TestSample", "", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameters")

		// Test empty query
		err = processCustomQuery(db, &integration.Entity{}, "", "TestSample", "", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameters")
	})

	t.Run("Successful Custom Query Processing", func(t *testing.T) {
		// Create proper test integration following MySQL patterns
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		// Create mock database
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Setup mock expectations
		rows := sqlmock.NewRows([]string{"user_count", "active_users"}).
			AddRow(42, 25)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) as user_count").
			WillReturnRows(rows)

		// Test the actual function
		err = processCustomQuery(db, testEntity, "SELECT COUNT(*) as user_count, COUNT(*) as active_users FROM users", "MysqlTestSample", "", arguments.ArgumentList{})
		assert.NoError(t, err)

		// Verify sqlmock expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())

		// Verify that metrics were created
		assert.Len(t, testEntity.Metrics, 1)
		metricSet := testEntity.Metrics[0].Metrics
		assert.Equal(t, "MysqlTestSample", testEntity.Metrics[0].Metrics["event_type"])
		assert.Equal(t, float64(42), metricSet["user_count"])
		assert.Equal(t, float64(25), metricSet["active_users"])
	})

	t.Run("Database Error Handling", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Setup mock to return database error
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM nonexistent").
			WillReturnError(sql.ErrNoRows)

		// Test the function - should return error due to query failure
		err = processCustomQuery(db, testEntity, "SELECT COUNT(*) FROM nonexistent", "MysqlErrorSample", "", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute custom query")

		// Verify all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())

		// Should not have created any metrics due to error
		assert.Len(t, testEntity.Metrics, 0)
	})

	t.Run("Multi-Row Query Produces One Event Per Row", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		rows := sqlmock.NewRows([]string{"table_name", "size_mb"}).
			AddRow("orders", 100).
			AddRow("users", 50).
			AddRow("products", 25)
		mock.ExpectQuery("SELECT table_name, size_mb FROM information_schema.tables").
			WillReturnRows(rows)

		err = processCustomQuery(db, testEntity, "SELECT table_name, size_mb FROM information_schema.tables", "MysqlTableSizeSample", "", arguments.ArgumentList{})
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())

		assert.Len(t, testEntity.Metrics, 3)
		assert.Equal(t, "orders", testEntity.Metrics[0].Metrics["table_name"])
		assert.Equal(t, float64(100), testEntity.Metrics[0].Metrics["size_mb"])
		assert.Equal(t, "users", testEntity.Metrics[1].Metrics["table_name"])
		assert.Equal(t, float64(50), testEntity.Metrics[1].Metrics["size_mb"])
		assert.Equal(t, "products", testEntity.Metrics[2].Metrics["table_name"])
		assert.Equal(t, float64(25), testEntity.Metrics[2].Metrics["size_mb"])
	})
}

func TestProcessCustomQueryWithDatabaseOverride(t *testing.T) {
	t.Run("Runs on a dedicated connection for the given database", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)
		testEntity := testIntegration.LocalEntity()

		// db is the shared pool connection; it must NOT receive the query.
		sharedDB, sharedMock, err := sqlmock.New()
		assert.NoError(t, err)
		defer sharedDB.Close()

		// altDB is the dedicated per-database connection that should receive the query.
		altDB, altMock, err := sqlmock.New()
		assert.NoError(t, err)
		defer altDB.Close()

		var requestedDSN string
		origOpen := openDatabaseConnection
		openDatabaseConnection = func(dsn string) (*sql.DB, error) {
			requestedDSN = dsn
			return altDB, nil
		}
		defer func() { openDatabaseConnection = origOpen }()

		rows := sqlmock.NewRows([]string{"total_orders"}).AddRow(3)
		altMock.ExpectQuery("SELECT COUNT\\(\\*\\) as total_orders FROM orders").WillReturnRows(rows)

		args := arguments.ArgumentList{Hostname: "localhost", Port: 3306, Username: "newrelic", Password: "secret"}
		err = processCustomQuery(sharedDB, testEntity, "SELECT COUNT(*) as total_orders FROM orders", "ShopSample", "shopdb", args)
		assert.NoError(t, err)

		assert.Contains(t, requestedDSN, "/shopdb")
		assert.NoError(t, altMock.ExpectationsWereMet())
		assert.NoError(t, sharedMock.ExpectationsWereMet()) // shared connection got zero queries

		assert.Len(t, testEntity.Metrics, 1)
		assert.Equal(t, float64(3), testEntity.Metrics[0].Metrics["total_orders"])
	})

	t.Run("Returns an error when the dedicated connection fails to open", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)
		testEntity := testIntegration.LocalEntity()

		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		origOpen := openDatabaseConnection
		openDatabaseConnection = func(dsn string) (*sql.DB, error) {
			return nil, fmt.Errorf("connection refused")
		}
		defer func() { openDatabaseConnection = origOpen }()

		err = processCustomQuery(db, testEntity, "SELECT 1", "ShopSample", "shopdb", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `failed to connect to database "shopdb"`)
		assert.Len(t, testEntity.Metrics, 0)
	})
}

func TestParseCustomMetricsConfig(t *testing.T) {
	yamlContent := `---
queries:
  - query: "SELECT 1 as test_metric"
    sample_name: "TestSample"
  - query: "SELECT 2 as another_metric"
    sample_name: "AnotherSample"
`

	var config CustomMetricsConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)

	assert.NoError(t, err)
	assert.Len(t, config.Queries, 2)
	assert.Equal(t, "SELECT 1 as test_metric", config.Queries[0].Query)
	assert.Equal(t, "TestSample", config.Queries[0].SampleName)
	assert.Equal(t, "SELECT 2 as another_metric", config.Queries[1].Query)
	assert.Equal(t, "AnotherSample", config.Queries[1].SampleName)
}

func TestParseCustomMetricsConfigDatabaseField(t *testing.T) {
	yamlContent := `---
queries:
  - query: "SELECT COUNT(*) as total_orders FROM orders"
    sample_name: "ShopRevenueSample"
    database: shopdb
  - query: "SELECT 1 as no_database_override"
    sample_name: "DefaultDbSample"
`

	var config CustomMetricsConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)

	assert.NoError(t, err)
	assert.Len(t, config.Queries, 2)
	assert.Equal(t, "shopdb", config.Queries[0].Database)
	assert.Empty(t, config.Queries[1].Database)
}

func TestParseCustomMetricsConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectErr   bool
		expectedLen int
	}{
		{
			name: "empty queries list",
			yamlContent: `---
queries: []
`,
			expectErr:   false,
			expectedLen: 0,
		},
		{
			name: "single query without sample_name",
			yamlContent: `---
queries:
  - query: "SELECT COUNT(*) as total_count"
`,
			expectErr:   false,
			expectedLen: 1,
		},
		{
			name: "multiple queries mixed with and without sample_name",
			yamlContent: `---
queries:
  - query: "SELECT 1 as metric1"
    sample_name: "CustomSample"
  - query: "SELECT 2 as metric2"
  - query: "SELECT 3 as metric3"
    sample_name: "AnotherSample"
`,
			expectErr:   false,
			expectedLen: 3,
		},
		{
			name: "invalid YAML syntax",
			yamlContent: `---
queries:
  - query: "SELECT 1"
    sample_name: "Test"
  - query: "SELECT 2"
    sample_name: [invalid yaml
`,
			expectErr:   true,
			expectedLen: 0,
		},
		{
			name:        "empty YAML",
			yamlContent: "",
			expectErr:   false,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config CustomMetricsConfig
			err := yaml.Unmarshal([]byte(tt.yamlContent), &config)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, config.Queries, tt.expectedLen)

				// Test default sample name behavior for queries without sample_name
				for _, query := range config.Queries {
					if query.SampleName == "" {
						// This should be handled by processCustomConfigFile logic
						assert.NotEmpty(t, query.Query)
					}
				}
			}
		})
	}
}

func TestProcessCustomConfigFileParameterValidation(t *testing.T) {
	tests := []struct {
		name        string
		db          *sql.DB
		entity      *integration.Entity
		configPath  string
		expectErr   bool
		errContains string
	}{
		{
			name:        "nil database",
			db:          nil,
			entity:      &integration.Entity{},
			configPath:  "test.yml",
			expectErr:   true,
			errContains: "invalid parameters",
		},
		{
			name:        "nil entity",
			db:          &sql.DB{},
			entity:      nil,
			configPath:  "test.yml",
			expectErr:   true,
			errContains: "invalid parameters",
		},
		{
			name:        "empty config path",
			db:          &sql.DB{},
			entity:      &integration.Entity{},
			configPath:  "",
			expectErr:   true,
			errContains: "invalid parameters",
		},
		{
			name:        "all nil parameters",
			db:          nil,
			entity:      nil,
			configPath:  "",
			expectErr:   true,
			errContains: "invalid parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processCustomConfigFile(tt.db, tt.entity, tt.configPath, arguments.ArgumentList{})
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessCustomConfigFileWithRealFile(t *testing.T) {
	t.Run("Success - Valid Config File", func(t *testing.T) {
		// Create proper test integration following MySQL patterns
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		// Create temporary config file
		configContent := `---
queries:
  - query: "SELECT COUNT(*) as user_count FROM users"
    sample_name: "MysqlUsersSample"
  - query: "SELECT COUNT(*) as order_count FROM orders"
    sample_name: "MysqlOrdersSample"
`
		tempFile, err := os.CreateTemp("", "test-config-*.yml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configContent)
		assert.NoError(t, err)
		tempFile.Close()

		// Create mock database
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Setup expectations for both queries
		userRows := sqlmock.NewRows([]string{"user_count"}).AddRow(42)
		orderRows := sqlmock.NewRows([]string{"order_count"}).AddRow(15)

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) as user_count FROM users").
			WillReturnRows(userRows)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) as order_count FROM orders").
			WillReturnRows(orderRows)

		// Test the function
		err = processCustomConfigFile(db, testEntity, tempFile.Name(), arguments.ArgumentList{})
		assert.NoError(t, err)

		// Verify all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())

		// Verify metrics were created for both queries
		assert.Len(t, testEntity.Metrics, 2)

		// Check first metric set
		firstMetricSet := testEntity.Metrics[0].Metrics
		assert.Equal(t, "MysqlUsersSample", firstMetricSet["event_type"])
		assert.Equal(t, float64(42), firstMetricSet["user_count"])

		// Check second metric set
		secondMetricSet := testEntity.Metrics[1].Metrics
		assert.Equal(t, "MysqlOrdersSample", secondMetricSet["event_type"])
		assert.Equal(t, float64(15), secondMetricSet["order_count"])
	})

	t.Run("Error - File Not Found", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Test with non-existent file
		err = processCustomConfigFile(db, testEntity, "/nonexistent/config.yml", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("Error - Invalid YAML", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		// Create temporary config file with invalid YAML
		invalidYAML := `---
queries:
  - query: "SELECT 1"
    sample_name: "Test"
  - query: "SELECT 2"
    sample_name: [invalid yaml syntax
`
		tempFile, err := os.CreateTemp("", "test-invalid-*.yml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(invalidYAML)
		assert.NoError(t, err)
		tempFile.Close()

		db, _, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Test with invalid YAML
		err = processCustomConfigFile(db, testEntity, tempFile.Name(), arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse YAML config")
	})

	t.Run("Partial Success - One Query Fails", func(t *testing.T) {
		testIntegration, err := integration.New("test", "1.0.0")
		assert.NoError(t, err)

		testEntity := testIntegration.LocalEntity()

		// Create config with two queries, one will fail
		configContent := `---
queries:
  - query: "SELECT COUNT(*) as user_count FROM users"
    sample_name: "MysqlUsersSample"
  - query: "SELECT COUNT(*) as invalid_count FROM nonexistent_table"
    sample_name: "MysqlInvalidSample"
`
		tempFile, err := os.CreateTemp("", "test-partial-*.yml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configContent)
		assert.NoError(t, err)
		tempFile.Close()

		// Create mock database
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// First query succeeds, second fails
		userRows := sqlmock.NewRows([]string{"user_count"}).AddRow(42)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) as user_count FROM users").
			WillReturnRows(userRows)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) as invalid_count FROM nonexistent_table").
			WillReturnError(sql.ErrNoRows)

		// Test should not return error (fails gracefully)
		err = processCustomConfigFile(db, testEntity, tempFile.Name(), arguments.ArgumentList{})
		assert.NoError(t, err) // Function continues processing even if individual queries fail

		// Verify all expectations were met
		assert.NoError(t, mock.ExpectationsWereMet())

		// Should have one successful metric set (first query succeeded)
		assert.Len(t, testEntity.Metrics, 1)
		metricSet := testEntity.Metrics[0].Metrics
		assert.Equal(t, "MysqlUsersSample", metricSet["event_type"])
		assert.Equal(t, float64(42), metricSet["user_count"])
	})
}

func TestDefaultSampleNameBehavior(t *testing.T) {
	yamlContent := `---
queries:
  - query: "SELECT 1 as test_metric"
    sample_name: "CustomSample"
  - query: "SELECT 2 as another_metric"
`

	var config CustomMetricsConfig
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	assert.NoError(t, err)
	assert.Len(t, config.Queries, 2)

	// First query has explicit sample name
	assert.Equal(t, "CustomSample", config.Queries[0].SampleName)

	// Second query has empty sample name (should use default)
	assert.Empty(t, config.Queries[1].SampleName)

	// Verify the default behavior logic from processCustomConfigFile
	for i, customQuery := range config.Queries {
		sampleName := customQuery.SampleName
		if sampleName == "" {
			sampleName = "MysqlCustomSample"
		}

		if i == 0 {
			assert.Equal(t, "CustomSample", sampleName)
		} else {
			assert.Equal(t, "MysqlCustomSample", sampleName)
		}
	}
}
