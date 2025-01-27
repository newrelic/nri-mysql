package validator

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
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

var errQuery = errors.New("query failed")

func TestCheckEssentialInstruments_AllEnabled(t *testing.T) {
	rows := sqlmock.NewRows([]string{"NAME", "ENABLED"}).
		AddRow("wait/synch/mutex/sql/LOCK_plugin", "YES").
		AddRow("statement/sql/select", "YES").
		AddRow("wait/io/file/sql/FILE", "YES")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(buildInstrumentQuery()).WillReturnRows(rows)
	err = checkEssentialInstruments(mockDataSource)
	assert.NoError(t, err)
}

func TestValidatePreconditions_PerformanceSchemaDisabled(t *testing.T) {
	rows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("performance_schema", "OFF")
	versionRows := sqlmock.NewRows([]string{"VERSION()"}).
		AddRow("8.0.23")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(rows)
	mock.ExpectQuery("SELECT VERSION();").WillReturnRows(versionRows)
	err = ValidatePreconditions(mockDataSource)
	assert.Error(t, err)
	assert.Equal(t, ErrPerformanceSchemaDisabled, err)
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
				mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnError(errQuery)
			},
			assertError: true,
		},
		{
			name: "EssentialInstrumentsCheckFailed",
			expectQueryFunc: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(buildInstrumentQuery()).WillReturnError(errQuery)
			},
			assertError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
				AddRow("performance_schema", "ON")
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()
			sqlxDB := sqlx.NewDb(db, "sqlmock")
			mockDataSource := &mockDataSource{db: sqlxDB}

			mock.ExpectQuery(performanceSchemaQuery).WillReturnRows(rows)
			tc.expectQueryFunc(mock) // Dynamically call the query expectation function
			err = ValidatePreconditions(mockDataSource)
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
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
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
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(buildConsumerStatusQuery()).WillReturnRows(rows)
	err = checkEssentialConsumers(mockDataSource)
	assert.Error(t, err)
}

func TestCheckEssentialInstruments_InstrumentNotEnabled(t *testing.T) {
	rows := sqlmock.NewRows([]string{"NAME", "ENABLED", "TIMED"}).
		AddRow("wait/synch/mutex/sql/LOCK_plugin", "NO", "YES")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery(buildInstrumentQuery()).WillReturnRows(rows)
	err = checkEssentialInstruments(mockDataSource)
	assert.Error(t, err)
}

func TestGetMySQLVersion(t *testing.T) {
	rows := sqlmock.NewRows([]string{"VERSION()"}).
		AddRow("8.0.23")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery("SELECT VERSION();").WillReturnRows(rows)
	version, err := getMySQLVersion(mockDataSource)
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
