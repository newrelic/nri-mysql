package performancemetricscollectors

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"

	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func convertNullString(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func convertNullInt64(ni sql.NullInt64) *int64 {
	if ni.Valid {
		return &ni.Int64
	}
	return nil
}

// Mocking utils.IngestMetric function
type MockUtilsIngest struct {
	mock.Mock
}

func (m *MockUtilsIngest) IngestMetric(metricList []interface{}, sampleName string, i *integration.Integration, args arguments.ArgumentList) error {
	callArgs := m.Called(metricList, sampleName, i, args)
	return callArgs.Error(0)
}

func Float64Ptr(f float64) *float64 {
	return &f
}

type dbWrapper struct {
	DB *sqlx.DB
}

func (d *dbWrapper) Close() {
	d.DB.Close()
}

func (d *dbWrapper) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return d.DB.QueryxContext(ctx, query, args...)
}

func (d *dbWrapper) QueryX(query string) (*sqlx.Rows, error) {
	return d.DB.Queryx(query)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPopulateBlockingSessionMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	excludedDatabases := []string{"mysql", "information_schema", "performance_schema", "sys"}
	queryCountThreshold := 10

	// Test case: Error preparing the SQL query
	t.Run("ErrorPreparingQuery", func(t *testing.T) {
		// Use a mock function for sqlx.In
		mockSqlxIn := func(_ string, _ ...interface{}) (string, []interface{}, error) {
			return "", nil, fmt.Errorf("mock error")
		}

		_, _, err := mockSqlxIn(utils.BlockingSessionsQuery, strings.Join(excludedDatabases, ","), min(queryCountThreshold, constants.MaxQueryCountThreshold))
		if err == nil {
			t.Fatal("Expected error preparing query, got nil")
		}
	})

	// Test case: Error collecting metrics
	t.Run("ErrorCollectingMetrics", func(t *testing.T) {
		query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, min(queryCountThreshold, constants.MaxQueryCountThreshold))
		assert.NoError(t, err)

		query = sqlxDB.Rebind(query)

		driverArgs := make([]driver.Value, len(inputArgs))
		for i, v := range inputArgs {
			driverArgs[i] = driver.Value(v)
		}
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnError(fmt.Errorf("query error"))

		dataSource := &dbWrapper{DB: sqlxDB}
		_, err = utils.CollectMetrics[utils.BlockingSessionMetrics](dataSource, query, inputArgs...)
		if err == nil {
			t.Fatal("Expected error collecting metrics, got nil")
		}
	})

	// Test case: No metrics collected
	t.Run("NoMetricsCollected", func(t *testing.T) {
		query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, min(queryCountThreshold, constants.MaxQueryCountThreshold))
		assert.NoError(t, err)

		query = sqlxDB.Rebind(query)
		driverArgs := make([]driver.Value, len(inputArgs))
		for i, v := range inputArgs {
			driverArgs[i] = driver.Value(v)
		}
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnRows(sqlmock.NewRows(nil))

		dataSource := &dbWrapper{DB: sqlxDB}
		metrics, err := utils.CollectMetrics[utils.BlockingSessionMetrics](dataSource, query, inputArgs...)
		assert.NoError(t, err)
		assert.Empty(t, metrics)
	})

	// Test case: Successful metrics collection
	t.Run("SuccessfulMetricsCollection", func(t *testing.T) {
		query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, min(queryCountThreshold, constants.MaxQueryCountThreshold))
		assert.NoError(t, err)

		query = sqlxDB.Rebind(query)
		driverArgs := make([]driver.Value, len(inputArgs))
		for i, v := range inputArgs {
			driverArgs[i] = driver.Value(v)
		}

		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnRows(sqlmock.NewRows([]string{
			"blocked_txn_id", "blocked_pid", "blocked_thread_id", "blocked_query_id", "blocked_query", "blocked_status", "blocked_host", "database_name", "blocking_txn_id", "blocking_pid", "blocking_thread_id", "blocking_status", "blocking_host", "blocking_query_id", "blocking_query",
		}).AddRow(
			"blocked_txn_id_1", "blocked_pid_1", 123, "blocked_query_id_1", "blocked_query_1", "blocked_status_1", "blocked_host_1", "database_name_1", "blocking_txn_id_1", "blocking_pid_1", 456, "blocking_status_1", "blocking_host_1", "blocking_query_id_1", "blocking_query_1",
		).AddRow(
			"blocked_txn_id_2", "blocked_pid_2", 124, "blocked_query_id_2", "blocked_query_2", "blocked_status_2", "blocked_host_2", "database_name_2", "blocking_txn_id_2", "blocking_pid_2", 457, "blocking_status_2", "blocking_host_2", "blocking_query_id_2", "blocking_query_2",
		))

		dataSource := &dbWrapper{DB: sqlxDB}
		metrics, err := utils.CollectMetrics[utils.BlockingSessionMetrics](dataSource, query, inputArgs...)
		assert.NoError(t, err)
		assert.Len(t, metrics, 2)
	})

	// Test case: PopulateBlockingSessionMetrics function
	t.Run("PopulateBlockingSessionMetrics", func(t *testing.T) {
		query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, min(queryCountThreshold, constants.MaxQueryCountThreshold))
		assert.NoError(t, err)

		query = sqlxDB.Rebind(query)
		driverArgs := make([]driver.Value, len(inputArgs))
		for i, v := range inputArgs {
			driverArgs[i] = driver.Value(v)
		}

		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnRows(sqlmock.NewRows([]string{
			"blocked_txn_id", "blocked_pid", "blocked_thread_id", "blocked_query_id", "blocked_query",
			"blocked_status", "blocked_host", "database_name", "blocking_txn_id", "blocking_pid",
			"blocking_thread_id", "blocking_status", "blocking_host", "blocking_query_id", "blocking_query",
		}).AddRow(
			"blocked_txn_id_1", "blocked_pid_1", 123, "blocked_query_id_1", "blocked_query_1",
			"blocked_status_1", "blocked_host_1", "database_name_1", "blocking_txn_id_1", "blocking_pid_1",
			456, "blocking_status_1", "blocking_host_1", "blocking_query_id_1", "blocking_query_1",
		).AddRow(
			"blocked_txn_id_2", "blocked_pid_2", 124, "blocked_query_id_2", "blocked_query_2",
			"blocked_status_2", "blocked_host_2", "database_name_2", "blocking_txn_id_2", "blocking_pid_2",
			457, "blocking_status_2", "blocking_host_2", "blocking_query_id_2", "blocking_query_2",
		))

		dataSource := &dbWrapper{DB: sqlxDB}
		i, _ := integration.New("test", "1.0.0")
		argList := arguments.ArgumentList{QueryCountThreshold: queryCountThreshold}

		PopulateBlockingSessionMetrics(dataSource, i, argList, excludedDatabases)

		assert.Len(t, i.LocalEntity().Metrics, 0)
	})
}

func TestSetBlockingQueryMetrics(t *testing.T) {
	i, err := integration.New("test", "1.0.0")
	assert.NoError(t, err)
	e := i.LocalEntity()
	args := arguments.ArgumentList{}
	metrics := []utils.BlockingSessionMetrics{
		{
			BlockedTxnID:     convertNullString(sql.NullString{String: "blocked_txn_id", Valid: true}),
			BlockedPID:       convertNullString(sql.NullString{String: "blocked_pid", Valid: true}),
			BlockedThreadID:  convertNullInt64(sql.NullInt64{Int64: 123, Valid: true}),
			BlockedQueryID:   convertNullString(sql.NullString{String: "blocked_query_id", Valid: true}),
			BlockedQuery:     convertNullString(sql.NullString{String: "blocked_query", Valid: true}),
			BlockedStatus:    convertNullString(sql.NullString{String: "blocked_status", Valid: true}),
			BlockedHost:      convertNullString(sql.NullString{String: "blocked_host", Valid: true}),
			BlockedDB:        convertNullString(sql.NullString{String: "blocked_db", Valid: true}),
			BlockingTxnID:    convertNullString(sql.NullString{String: "blocking_txn_id", Valid: true}),
			BlockingPID:      convertNullString(sql.NullString{String: "blocking_pid", Valid: true}),
			BlockingThreadID: convertNullInt64(sql.NullInt64{Int64: 456, Valid: true}),
			BlockingStatus:   convertNullString(sql.NullString{String: "blocking_status", Valid: true}),
			BlockingHost:     convertNullString(sql.NullString{String: "blocking_host", Valid: true}),
			BlockingQueryID:  convertNullString(sql.NullString{String: "blocking_query_id", Valid: true}),
			BlockingQuery:    convertNullString(sql.NullString{String: "blocking_query", Valid: true}),
		},
	}
	err = setBlockingQueryMetrics(metrics, i, args)
	assert.NoError(t, err)
	ms := e.Metrics[0]
	assert.Equal(t, "blocked_txn_id", ms.Metrics["blocked_txn_id"])
	assert.Equal(t, "blocked_pid", ms.Metrics["blocked_pid"])
	assert.Equal(t, float64(123), ms.Metrics["blocked_thread_id"])
	assert.Equal(t, "blocked_query_id", ms.Metrics["blocked_query_id"])
	assert.Equal(t, "blocked_query", ms.Metrics["blocked_query"])
	assert.Equal(t, "blocked_host", ms.Metrics["blocked_host"])
	assert.Equal(t, "blocked_db", ms.Metrics["database_name"])
	assert.Equal(t, "blocking_txn_id", ms.Metrics["blocking_txn_id"])
	assert.Equal(t, "blocking_pid", ms.Metrics["blocking_pid"])
	assert.Equal(t, float64(456), ms.Metrics["blocking_thread_id"])
	assert.Equal(t, "blocking_host", ms.Metrics["blocking_host"])
	assert.Equal(t, "blocking_query_id", ms.Metrics["blocking_query_id"])
	assert.Equal(t, "blocking_query", ms.Metrics["blocking_query"])
}
