package validator

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	arguments "github.com/newrelic/nri-mysql/src/args"
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

	mock.ExpectQuery("SELECT NAME, ENABLED FROM performance_schema.setup_instruments WHERE NAME LIKE 'wait/%' OR NAME LIKE 'statement/%' OR NAME LIKE '%lock%';").WillReturnRows(rows)
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

	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'performance_schema';").WillReturnRows(rows)
	mock.ExpectQuery("SELECT VERSION();").WillReturnRows(versionRows)
	err = ValidatePreconditions(mockDataSource)
	assert.Error(t, err)
	assert.Equal(t, ErrPerformanceSchemaDisabled, err)
}

func TestValidatePreconditions_EssentialConsumersCheckFailed(t *testing.T) {
	rows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("performance_schema", "ON")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'performance_schema';").WillReturnRows(rows)
	mock.ExpectQuery("SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current', 'events_waits_history_long', 'events_waits_history', 'events_statements_history_long', 'events_statements_history', 'events_statements_current', 'events_statements_cpu', 'events_transactions_current', 'events_stages_current');").WillReturnError(errQuery)
	err = ValidatePreconditions(mockDataSource)
	assert.Error(t, err)
}

func TestValidatePreconditions_EssentialInstrumentsCheckFailed(t *testing.T) {
	rows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("performance_schema", "ON")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'performance_schema';").WillReturnRows(rows)
	mock.ExpectQuery("SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current', 'events_waits_history_long', 'events_waits_history', 'events_statements_history_long', 'events_statements_history', 'events_statements_current', 'events_statements_cpu', 'events_transactions_current', 'events_stages_current');").WillReturnRows(sqlmock.NewRows([]string{"NAME", "ENABLED"}).AddRow("events_waits_current", "YES"))
	mock.ExpectQuery("SELECT NAME, ENABLED, TIMED FROM performance_schema.setup_instruments WHERE NAME LIKE 'wait/%' OR NAME LIKE 'statement/%' OR NAME LIKE '%lock%';").WillReturnError(errQuery)
	err = ValidatePreconditions(mockDataSource)
	assert.Error(t, err)
}

func TestIsPerformanceSchemaEnabled_NoRowsFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDataSource := &mockDataSource{db: sqlxDB}

	mock.ExpectQuery("SHOW GLOBAL VARIABLES LIKE 'performance_schema';").WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}))
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

	mock.ExpectQuery("SELECT NAME, ENABLED FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current', 'events_waits_history_long', 'events_waits_history', 'events_statements_history_long', 'events_statements_history', 'events_statements_current', 'events_statements_cpu', 'events_transactions_current', 'events_stages_current');").WillReturnRows(rows)
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

	mock.ExpectQuery("SELECT NAME, ENABLED, TIMED FROM performance_schema.setup_instruments WHERE NAME LIKE 'wait/%' OR NAME LIKE 'statement/%' OR NAME LIKE '%lock%';").WillReturnRows(rows)
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
	assert.False(t, isVersion8OrGreater("5.7.31"))
}

func TestParseVersion(t *testing.T) {
	major := parseVersion("8.0.23")
	assert.Equal(t, 8, major)

	major = parseVersion("5.7.31")
	assert.Equal(t, 5, major)

	major = parseVersion("invalid.version")
	assert.Equal(t, 0, major)
}

func TestValidateAndSetDefaults(t *testing.T) {
	tests := []struct {
		name                      string
		input                     arguments.ArgumentList
		expectedQueryResponseTime int
		expectedQueryCount        int
	}{
		{
			name: "Negative QueryResponseTimeThreshold",
			input: arguments.ArgumentList{
				QueryResponseTimeThreshold: -1,
				QueryCountThreshold:        10,
			},
			expectedQueryResponseTime: constants.DefaultQueryResponseTimeThreshold,
			expectedQueryCount:        10,
		},
		{
			name: "Negative QueryCountThreshold",
			input: arguments.ArgumentList{
				QueryResponseTimeThreshold: 10,
				QueryCountThreshold:        -1,
			},
			expectedQueryResponseTime: 10,
			expectedQueryCount:        constants.DefaultQueryCountThreshold,
		},
		{
			name: "QueryCountThreshold greater than max",
			input: arguments.ArgumentList{
				QueryResponseTimeThreshold: 10,
				QueryCountThreshold:        constants.MaxQueryCountThreshold + 1,
			},
			expectedQueryResponseTime: 10,
			expectedQueryCount:        constants.MaxQueryCountThreshold,
		},
		{
			name: "Valid thresholds",
			input: arguments.ArgumentList{
				QueryResponseTimeThreshold: 10,
				QueryCountThreshold:        5,
			},
			expectedQueryResponseTime: 10,
			expectedQueryCount:        5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ValidateAndSetDefaults(&tt.input)
			assert.Equal(t, tt.expectedQueryResponseTime, tt.input.QueryResponseTimeThreshold)
			assert.Equal(t, tt.expectedQueryCount, tt.input.QueryCountThreshold)
		})
	}
}
