package validator

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
)

type mockDataSource struct {
	db *sqlx.DB
}

func (m *mockDataSource) Close() {
	m.db.Close()
}

func (m *mockDataSource) QueryX(query string) (*sqlx.Rows, error) {
	return m.db.Queryx(query)
}

func (m *mockDataSource) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return m.db.QueryxContext(ctx, query, args...)
}

var errQueryFailed = errors.New("query failed")
var errQuery = errors.New("query error")
var errProcedure = errors.New("procedure error")
var errProcedureNotExist = errors.New("procedure newrelic.enable_essential_consumers_and_instruments does not exist")

func TestValidatePreconditions_PerformanceSchemaDisabled(t *testing.T) {
	rows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("performance_schema", "OFF")
	versionRows := sqlmock.NewRows([]string{"VERSION()"}).
		AddRow("8.0.23")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// Set the correct order of mock expectations
	mock.ExpectQuery(versionQuery).WillReturnRows(versionRows)
	mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(rows)

	_, err = ValidatePreconditions(mockDataSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "performance schema is not enabled")

	// Ensure all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidatePreconditions_EssentialChecksFailed(t *testing.T) {
	testCases := []struct {
		name            string
		expectQueryFunc func(mock sqlmock.Sqlmock)
		assertError     bool
	}{
		{
			name: "EssentialConsumersCheckFailed",
			expectQueryFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnError(errQueryFailed)
			},
			assertError: false, // The function logs a warning but does not return an error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			versionRows := sqlmock.NewRows([]string{"version"}).AddRow("8.0.23")
			performanceSchemaRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).AddRow("performance_schema", "ON")
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err, "an error was not expected when opening a stub database connection")
			defer db.Close()
			sqlxDB := sqlx.NewDb(db, "sqlmock")
			mockDataSource := &mockDataSource{db: sqlxDB}

			mock.ExpectQuery(versionQuery).WillReturnRows(versionRows)
			mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(performanceSchemaRows)
			tc.expectQueryFunc(mock) // Dynamically call the query expectation function

			_, err = ValidatePreconditions(mockDataSource)
			if tc.assertError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPerformanceSchemaEnabled_NoRowsFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}))
	enabled, err := isPerformanceSchemaEnabled(mockDataSource)
	assert.Error(t, err)
	assert.Equal(t, ErrNoRowsFound, err)
	assert.False(t, enabled)
}

func TestCheckEssentialConsumers_ConsumerNotEnabled(t *testing.T) {
	rows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
		AddRow("events_waits_current", "NO")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnRows(rows)
	err = checkAndEnableEssentialConsumers(mockDataSource)
	assert.Error(t, err)
}

func TestNumberOfEssentialConsumersEnabledCheck(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(mock sqlmock.Sqlmock)
		testFunc       func(dataSource *mockDataSource) (interface{}, error)
		expectedResult interface{}
		expectError    bool
	}{
		{
			name: "AllEssentialConsumersEnabled_Success",
			setupMock: func(mock sqlmock.Sqlmock) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				rows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
					AddRow("events_waits_current", "YES").
					AddRow("events_statements_history", "YES")
				mock.ExpectQuery(query).WillReturnRows(rows)
			},
			testFunc: func(dataSource *mockDataSource) (interface{}, error) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				return numberOfEssentialConsumersEnabled(dataSource, query)
			},
			expectedResult: 2,
			expectError:    false,
		},
		{
			name: "Failure_DatabaseQueryError",
			setupMock: func(mock sqlmock.Sqlmock) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				mock.ExpectQuery(query).WillReturnError(errQuery)
			},
			testFunc: func(dataSource *mockDataSource) (interface{}, error) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				return numberOfEssentialConsumersEnabled(dataSource, query)
			},
			expectedResult: 0,
			expectError:    true,
		},
		{
			name: "Failure_RowScanError",
			setupMock: func(mock sqlmock.Sqlmock) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				rows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
					AddRow("events_waits_current", nil) // Simulate scan error
				mock.ExpectQuery(query).WillReturnRows(rows)
			},
			testFunc: func(dataSource *mockDataSource) (interface{}, error) {
				query := "SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN (.+);"
				return numberOfEssentialConsumersEnabled(dataSource, query)
			},
			expectedResult: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			mockDataSource := &mockDataSource{db: sqlxDB}

			tt.setupMock(mock)

			result, err := tt.testFunc(mockDataSource)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEnableEssentialConsumersAndInstruments_FallbackToQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// Mock stored procedure to fail with a recognizable recoverable error
	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnError(errProcedureNotExist)

	// Mock explicit queries to succeed for fallback
	for _, query := range QueriesToEnableEssentialConsumersAndInstruments {
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("success"))
	}

	// Test that fallback to explicit queries works when stored procedure fails
	err = enableEssentialConsumersAndInstruments(mockDataSource)
	assert.NoError(t, err, "Should succeed with fallback mechanism")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableEssentialConsumersAndInstruments_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnRows(
		sqlmock.NewRows([]string{"result"}).AddRow("success"))

	err = enableEssentialConsumersAndInstruments(mockDataSource)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableEssentialConsumersAndInstruments_BothMethodsFail(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// Stored procedure fails with a recoverable error to trigger fallback
	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnError(errProcedureNotExist)

	// First explicit query fails
	mock.ExpectQuery(QueriesToEnableEssentialConsumersAndInstruments[0]).WillReturnError(errQuery)

	err = enableEssentialConsumersAndInstruments(mockDataSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute query")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableViaStoredProcedure_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnRows(
		sqlmock.NewRows([]string{"success"}).AddRow("1"))

	err = enableViaStoredProcedure(mockDataSource)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableViaStoredProcedure_Failure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnError(errProcedure)

	err = enableViaStoredProcedure(mockDataSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute stored procedure")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableViaExplicitQueries_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	for _, query := range QueriesToEnableEssentialConsumersAndInstruments {
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("success"))
	}

	err = enableViaExplicitQueries(mockDataSource)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableViaExplicitQueries_PartialFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// First query succeeds
	mock.ExpectQuery(QueriesToEnableEssentialConsumersAndInstruments[0]).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("success"))

	// Second query fails
	mock.ExpectQuery(QueriesToEnableEssentialConsumersAndInstruments[1]).WillReturnError(errQuery)

	err = enableViaExplicitQueries(mockDataSource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute query")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnableEssentialConsumersAndInstruments_ViaExplicitQueries(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// Mock stored procedure to fail with a recognizable recoverable error to trigger fallback
	mock.ExpectQuery(enableEssentialConsumersAndInstrumentsProcedureQuery).WillReturnError(errProcedureNotExist)

	// Mock the explicit queries to succeed
	for _, query := range QueriesToEnableEssentialConsumersAndInstruments {
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("success"))
	}

	err = enableEssentialConsumersAndInstruments(mockDataSource)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDatabaseVersion(t *testing.T) {
	rows := sqlmock.NewRows([]string{"VERSION()"}).
		AddRow("8.0.23")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(versionQuery).WillReturnRows(rows)
	version, err := getDatabaseVersion(mockDataSource)
	assert.NoError(t, err)
	assert.Equal(t, "8.0.23", version)
}
func TestIsVersion8OrGreater(t *testing.T) {
	assert.True(t, isVersion8OrGreater("8.0.23"))
	assert.True(t, isVersion8OrGreater("8.4"))
	assert.False(t, isVersion8OrGreater("5.7.31"))
	assert.False(t, isVersion8OrGreater("5.6"))
	assert.False(t, isVersion8OrGreater("5"))
	assert.False(t, isVersion8OrGreater("invalid.version.string"))
	assert.False(t, isVersion8OrGreater(""))
}

func TestExtractMajorFromVersion(t *testing.T) {
	major, err := extractMajorFromVersion("8.0.23")
	assert.NoError(t, err)
	assert.Equal(t, 8, major)

	major, err = extractMajorFromVersion("5.7.31")
	assert.NoError(t, err)
	assert.Equal(t, 5, major)

	major, err = extractMajorFromVersion("5")
	assert.Error(t, err)
	assert.Equal(t, 0, major)

	major, err = extractMajorFromVersion("invalid.version")
	assert.Error(t, err)
	assert.Equal(t, 0, major)

	major, err = extractMajorFromVersion("")
	assert.Error(t, err)
	assert.Equal(t, 0, major)
}

func TestGetValidSlowQueryFetchIntervalThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		expected  int
	}{
		{"Negative threshold", -1, constants.DefaultSlowQueryFetchInterval},
		{"Zero threshold", 0, 0},
		{"Positive threshold", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetValidSlowQueryFetchIntervalThreshold(tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValidQueryResponseTimeThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		expected  int
	}{
		{"Negative threshold", -1, constants.DefaultQueryResponseTimeThreshold},
		{"Zero threshold", 0, 0},
		{"Positive threshold", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetValidQueryResponseTimeThreshold(tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValidQueryCountThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		expected  int
	}{
		{"Negative threshold", -1, constants.DefaultQueryCountThreshold},
		{"Zero threshold", 0, 0},
		{"Threshold greater than max", constants.MaxQueryCountThreshold + 1, constants.MaxQueryCountThreshold},
		{"Threshold equal to max", constants.MaxQueryCountThreshold, constants.MaxQueryCountThreshold},
		{"Positive threshold", constants.MaxQueryCountThreshold - 1, constants.MaxQueryCountThreshold - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetValidQueryCountThreshold(tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MariaDB-specific validation tests
func TestValidatePreconditions_MariaDB(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		performanceOn  bool
		consumersEnabled int
		expectError    bool
		expectedProfile utils.DatabaseProfile
		expectZeroValue bool
	}{
		{
			name:          "MariaDB 10.6 with performance schema enabled",
			version:       "10.6.0-MariaDB",
			performanceOn: true,
			consumersEnabled: 5,
			expectError:   false,
			expectedProfile: utils.DatabaseProfile{
				Flavor:     utils.DatabaseFlavorMariaDB,
				RawVersion: "10.6.0-MariaDB",
			},
			expectZeroValue: false,
		},
		{
			name:          "MariaDB 10.5 with performance schema enabled",
			version:       "10.5.12-MariaDB-0ubuntu0.20.04.1",
			performanceOn: true,
			consumersEnabled: 5,
			expectError:   false,
			expectedProfile: utils.DatabaseProfile{
				Flavor:     utils.DatabaseFlavorMariaDB,
				RawVersion: "10.5.12-MariaDB-0ubuntu0.20.04.1",
			},
			expectZeroValue: false,
		},
		{
			name:          "MariaDB 10.2 minimum supported version",
			version:       "10.2.0-MariaDB",
			performanceOn: true,
			consumersEnabled: 5,
			expectError:   false,
			expectedProfile: utils.DatabaseProfile{
				Flavor:     utils.DatabaseFlavorMariaDB,
				RawVersion: "10.2.0-MariaDB",
			},
			expectZeroValue: false,
		},
		{
			name:          "MariaDB 11.x future major version",
			version:       "11.0.2-MariaDB",
			performanceOn: true,
			consumersEnabled: 5,
			expectError:   false,
			expectedProfile: utils.DatabaseProfile{
				Flavor:     utils.DatabaseFlavorMariaDB,
				RawVersion: "11.0.2-MariaDB",
			},
			expectZeroValue: false,
		},
		{
			name:            "MariaDB 10.1 unsupported version",
			version:         "10.1.48-MariaDB",
			performanceOn:   false,
			expectError:     true,
			expectZeroValue: true,
		},
		{
			name:            "MariaDB 10.0 unsupported version",
			version:         "10.0.38-MariaDB",
			performanceOn:   false,
			expectError:     true,
			expectZeroValue: true,
		},
		{
			name:          "MariaDB with performance schema disabled",
			version:       "10.6.0-MariaDB",
			performanceOn: false,
			expectError:   true,
			expectZeroValue: true,
		},
		{
			name:          "MySQL 8.0 for comparison",
			version:       "8.0.23",
			performanceOn: true,
			consumersEnabled: 5,
			expectError:   false,
			expectedProfile: utils.DatabaseProfile{
				Flavor:     utils.DatabaseFlavorMySQL,
				RawVersion: "8.0.23",
			},
			expectZeroValue: false,
		},
		{
			name:          "MySQL 5.7 should fail version check",
			version:       "5.7.31",
			performanceOn: false, // Doesn't matter - version check fails first
			expectError:   true,
			expectZeroValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			mockDataSource := &mockDataSource{db: sqlxDB}

			// Setup version query
			versionRows := sqlmock.NewRows([]string{"VERSION()"}).AddRow(tt.version)
			mock.ExpectQuery(versionQuery).WillReturnRows(versionRows)

			// Unsupported versions fail the version check early, so no further queries expected
			unsupportedVersions := map[string]bool{
				"5.7.31":          true,
				"10.1.48-MariaDB": true,
				"10.0.38-MariaDB": true,
			}
			if unsupportedVersions[tt.version] {
				// No further mock expectations - function returns early
			} else {
				// Setup performance schema query if needed
				if tt.performanceOn {
					performanceRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
						AddRow("performance_schema", "ON")
					mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(performanceRows)

					// Setup consumer status query for all database types (function always calls this)
					consumerRows := sqlmock.NewRows([]string{"NAME", "ENABLED"})
					for i := 0; i < tt.consumersEnabled; i++ {
						consumerRows.AddRow("events_waits_current", "YES")
					}
					mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnRows(consumerRows)
				} else {
					performanceRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
						AddRow("performance_schema", "OFF")
					mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(performanceRows)
				}
			}

			// Execute test
			profile, err := ValidatePreconditions(mockDataSource)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectZeroValue {
					assert.Equal(t, utils.DatabaseProfile{}, profile)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedProfile, profile)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDatabaseFlavorDetection_ValidatePreconditions(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		expectedFlavor  utils.DatabaseFlavor
	}{
		{
			name:           "MariaDB detection",
			version:        "10.6.0-MariaDB",
			expectedFlavor: utils.DatabaseFlavorMariaDB,
		},
		{
			name:           "MySQL detection",
			version:        "8.0.23",
			expectedFlavor: utils.DatabaseFlavorMySQL,
		},
		{
			name:           "Case insensitive MariaDB detection",
			version:        "10.6.0-mariadb",
			expectedFlavor: utils.DatabaseFlavorMariaDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			mockDataSource := &mockDataSource{db: sqlxDB}

			// Setup successful validation path
			versionRows := sqlmock.NewRows([]string{"VERSION()"}).AddRow(tt.version)
			mock.ExpectQuery(versionQuery).WillReturnRows(versionRows)

			performanceRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
				AddRow("performance_schema", "ON")
			mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(performanceRows)

			// Consumer check is called for all database types
			consumerRows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
				AddRow("events_waits_current", "YES").
				AddRow("events_statements_history", "YES").
				AddRow("events_statements_current", "YES").
				AddRow("events_statements_history_long", "YES").
				AddRow("events_waits_history", "YES")
			mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnRows(consumerRows)

			profile, err := ValidatePreconditions(mockDataSource)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFlavor, profile.Flavor)
			assert.Equal(t, tt.version, profile.RawVersion)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestIsMariaDBVersionSupported(t *testing.T) {
	tests := []struct {
		version   string
		supported bool
	}{
		{"10.2.0-MariaDB", true},   // exact minimum boundary
		{"10.2.44-MariaDB", true},  // patch release at minimum minor
		{"10.3.0-MariaDB", true},   // above minimum
		{"10.6.12-MariaDB", true},  // common production version
		{"11.0.2-MariaDB", true},   // future major version
		{"11.4.0-MariaDB", true},   // future major, higher minor
		{"10.1.48-MariaDB", false}, // one minor below minimum
		{"10.0.38-MariaDB", false}, // well below minimum
		// edge case: minor suffix style seen on some distros
		{"10.2-MariaDB", true},
		{"10.1-MariaDB", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.supported, isMariaDBVersionSupported(tt.version))
		})
	}
}

func TestMariaDBVersionValidation(t *testing.T) {
	// Test that MariaDB doesn't go through MySQL version validation
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	// MariaDB version that would fail MySQL validation
	mariadbVersion := "10.3.0-MariaDB" // This would be < 8.0 for MySQL

	versionRows := sqlmock.NewRows([]string{"VERSION()"}).AddRow(mariadbVersion)
	mock.ExpectQuery(versionQuery).WillReturnRows(versionRows)

	performanceRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("performance_schema", "ON")
	mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(performanceRows)

	// Consumer status check is always called
	consumerRows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
		AddRow("events_waits_current", "YES").
		AddRow("events_statements_history", "YES").
		AddRow("events_statements_current", "YES").
		AddRow("events_statements_history_long", "YES").
		AddRow("events_waits_history", "YES")
	mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnRows(consumerRows)

	profile, err := ValidatePreconditions(mockDataSource)

	// Should succeed for MariaDB even with "old" version number
	assert.NoError(t, err)
	assert.Equal(t, utils.DatabaseFlavorMariaDB, profile.Flavor)
	assert.Equal(t, mariadbVersion, profile.RawVersion)

	assert.NoError(t, mock.ExpectationsWereMet())
}
