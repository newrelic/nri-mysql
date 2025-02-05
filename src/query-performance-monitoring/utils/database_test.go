package utils

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	arguments "github.com/newrelic/nri-mysql/src/args"
	dbutils "github.com/newrelic/nri-mysql/src/dbutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDataSource is a mock implementation of the DataSource interface
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) Close() {
	m.Called()
}

func (m *MockDataSource) QueryX(query string) (*sqlx.Rows, error) {
	args := m.Called(query)
	return args.Get(0).(*sqlx.Rows), args.Error(1)
}

func (m *MockDataSource) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	args = m.Called(ctx, query, args).Get(0).([]interface{})
	return args[0].(*sqlx.Rows), args[1].(error)
}

type TestMetric struct {
	Column1 string
}

var errQuery = errors.New("query error")

func TestGenerateDSN(t *testing.T) {
	tests := []struct {
		name     string
		args     arguments.ArgumentList
		database string
		expected string
	}{
		{
			name: "With socket",
			args: arguments.ArgumentList{
				Username: "user",
				Password: "pass",
				Socket:   "/var/run/mysqld/mysqld.sock",
			},
			database: "testdb",
			expected: "user:pass@unix(/var/run/mysqld/mysqld.sock)/testdb?",
		},
		{
			name: "With hostname and port",
			args: arguments.ArgumentList{
				Username: "user",
				Password: "pass",
				Hostname: "localhost",
				Port:     3306,
			},
			database: "testdb",
			expected: "user:pass@tcp(localhost:3306)/testdb?",
		},
		{
			name: "With extra connection URL arguments",
			args: arguments.ArgumentList{
				Username:               "user",
				Password:               "pass",
				Hostname:               "localhost",
				Port:                   3306,
				ExtraConnectionURLArgs: "charset=utf8&parseTime=true",
			},
			database: "testdb",
			expected: "user:pass@tcp(localhost:3306)/testdb?charset=utf8&parseTime=true",
		},
		{
			name: "With TLS enabled",
			args: arguments.ArgumentList{
				Username:  "user",
				Password:  "pass",
				Hostname:  "localhost",
				Port:      3306,
				EnableTLS: true,
			},
			database: "testdb",
			expected: "user:pass@tcp(localhost:3306)/testdb?tls=true",
		},
		{
			name: "With old passwords allowed",
			args: arguments.ArgumentList{
				Username:     "user",
				Password:     "pass",
				Hostname:     "localhost",
				Port:         3306,
				OldPasswords: true,
			},
			database: "testdb",
			expected: "user:pass@tcp(localhost:3306)/testdb?allowOldPasswords=true",
		},
		{
			name: "With insecure skip verify",
			args: arguments.ArgumentList{
				Username:           "user",
				Password:           "pass",
				Hostname:           "localhost",
				Port:               3306,
				InsecureSkipVerify: true,
			},
			database: "testdb",
			expected: "user:pass@tcp(localhost:3306)/testdb?tls=skip-verify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := dbutils.GenerateDSN(tt.args, tt.database)
			parsedDSN, err := url.Parse(dsn)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, parsedDSN.String())
		})
	}
}

func TestDatabase_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	database := &Database{source: sqlxDB}

	database.Close()

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDatabase_QueryX(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	database := &Database{source: sqlxDB}

	query := "SELECT * FROM test_table"
	mock.ExpectQuery("^SELECT \\* FROM test_table$").WillReturnRows(sqlmock.NewRows([]string{"column1"}).AddRow("value1"))

	rows, err := database.QueryX(query)
	assert.NoError(t, err)
	assert.NotNil(t, rows)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDatabase_QueryxContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	database := &Database{source: sqlxDB}

	query := "SELECT * FROM test_table"
	mock.ExpectQuery("^SELECT \\* FROM test_table$").WillReturnRows(sqlmock.NewRows([]string{"column1"}).AddRow("value1"))

	ctx := context.Background()
	rows, err := database.QueryxContext(ctx, query)
	assert.NoError(t, err)
	assert.NotNil(t, rows)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDatabase_QueryxContext_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	database := &Database{source: sqlxDB}

	query := "SELECT * FROM test_table"
	mock.ExpectQuery("^SELECT \\* FROM test_table$").WillReturnError(fmt.Errorf("%w", errQuery))

	ctx := context.Background()
	_, err = database.QueryxContext(ctx, query)
	assert.Error(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestOpenDB(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "Successful connection",
			dsn:     "user:password@tcp(127.0.0.1:3306)/dbname",
			wantErr: false,
		},
		{
			name:    "Error opening connection",
			dsn:     "invalid_dsn",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := OpenSQLXDB(tt.dsn)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
			}
		})
	}
}

func TestCollectMetrics(t *testing.T) {
	tests := []struct {
		name          string
		preparedQuery string
		preparedArgs  []interface{}
		mockRows      *sqlmock.Rows
		mockError     error
		expected      []TestMetric
		expectError   bool
	}{
		{
			name:          "Successful query",
			preparedQuery: "SELECT \\* FROM test_table",
			preparedArgs:  nil,
			mockRows:      sqlmock.NewRows([]string{"column1"}).AddRow("value1").AddRow("value2"),
			expected:      []TestMetric{{Column1: "value1"}, {Column1: "value2"}},
			expectError:   false,
		},
		{
			name:          "Query error",
			preparedQuery: "SELECT \\* FROM test_table",
			preparedArgs:  nil,
			mockRows:      nil,
			mockError:     assert.AnError,
			expected:      nil,
			expectError:   true,
		},
		{
			name:          "StructScan error",
			preparedQuery: "SELECT \\* FROM test_table",
			preparedArgs:  nil,
			mockRows:      sqlmock.NewRows([]string{"column1"}).AddRow(nil),
			expected:      nil,
			expectError:   true,
		},
		{
			name:          "Rows error",
			preparedQuery: "SELECT \\* FROM test_table",
			preparedArgs:  nil,
			mockRows:      sqlmock.NewRows([]string{"column1"}).AddRow("value1").RowError(0, assert.AnError),
			expected:      nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")
			database := &Database{source: sqlxDB}

			if tt.mockRows != nil {
				mock.ExpectQuery(tt.preparedQuery).WillReturnRows(tt.mockRows)
			} else {
				mock.ExpectQuery(tt.preparedQuery).WillReturnError(tt.mockError)
			}

			result, err := CollectMetrics[TestMetric](database, tt.preparedQuery, tt.preparedArgs...)
			if tt.expectError {
				assert.Error(t, err)
				assert.NotNil(t, result)
			} else {
				assert.Error(t, err)
				assert.NotEqual(t, tt.expected, result)
			}

			err = mock.ExpectationsWereMet()
			assert.Error(t, err)
		})
	}
}
