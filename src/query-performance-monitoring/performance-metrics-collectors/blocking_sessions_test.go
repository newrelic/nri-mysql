package performancemetricscollectors

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"

	arguments "github.com/newrelic/nri-mysql/src/args"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	errQuery = errors.New("query error")
)

// ptr returns a pointer to the value passed in.
func ptr[T any](v T) *T {
	return &v
}

// Mocking utils.IngestMetric function
type MockUtilsIngest struct {
	mock.Mock
}

func (m *MockUtilsIngest) IngestMetric(metricList []interface{}, sampleName string, i *integration.Integration, args arguments.ArgumentList) error {
	callArgs := m.Called(metricList, sampleName, i, args)
	return callArgs.Error(0)
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

func TestPopulateBlockingSessionMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	excludedDatabases := []string{"mysql", "information_schema", "performance_schema", "sys"}
	queryCountThreshold := 10

	t.Run("ErrorCollectingMetrics", func(t *testing.T) {
		testErrorCollectingMetrics(t, sqlxDB, mock, excludedDatabases, queryCountThreshold)
	})

	t.Run("NoMetricsCollected", func(t *testing.T) {
		testNoMetricsCollected(t, sqlxDB, mock, excludedDatabases, queryCountThreshold)
	})

	t.Run("SuccessfulMetricsCollection", func(t *testing.T) {
		testSuccessfulMetricsCollection(t, sqlxDB, mock, excludedDatabases, queryCountThreshold)
	})

	t.Run("PopulateBlockingSessionMetrics", func(t *testing.T) {
		testPopulateBlockingSessionMetrics(t, sqlxDB, mock, excludedDatabases, queryCountThreshold)
	})
}

func testErrorCollectingMetrics(t *testing.T, sqlxDB *sqlx.DB, mock sqlmock.Sqlmock, excludedDatabases []string, queryCountThreshold int) {
	query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, queryCountThreshold)
	assert.NoError(t, err)

	query = sqlxDB.Rebind(query)

	driverArgs := make([]driver.Value, len(inputArgs))
	for i, v := range inputArgs {
		driverArgs[i] = driver.Value(v)
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnError(errQuery)

	dataSource := &dbWrapper{DB: sqlxDB}
	_, err = utils.CollectMetrics[utils.BlockingSessionMetrics](dataSource, query, inputArgs...)
	assert.Error(t, err, "Expected error collecting metrics, got nil")
}

func testNoMetricsCollected(t *testing.T, sqlxDB *sqlx.DB, mock sqlmock.Sqlmock, excludedDatabases []string, queryCountThreshold int) {
	query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, queryCountThreshold)
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
}

func testSuccessfulMetricsCollection(t *testing.T, sqlxDB *sqlx.DB, mock sqlmock.Sqlmock, excludedDatabases []string, queryCountThreshold int) {
	query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, queryCountThreshold)
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
}

func testPopulateBlockingSessionMetrics(t *testing.T, sqlxDB *sqlx.DB, mock sqlmock.Sqlmock, excludedDatabases []string, queryCountThreshold int) {
	query, inputArgs, err := sqlx.In(utils.BlockingSessionsQuery, excludedDatabases, queryCountThreshold)
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
}

func TestSetBlockingQueryMetrics(t *testing.T) {
	i, err := integration.New("test", "1.0.0")
	assert.NoError(t, err)
	e := i.LocalEntity()
	args := arguments.ArgumentList{}
	metrics := []utils.BlockingSessionMetrics{
		{
			BlockedTxnID:     ptr("blocked_txn_id"),
			BlockedPID:       ptr("blocked_pid"),
			BlockedThreadID:  ptr(int64(123)),
			BlockedQueryID:   ptr("blocked_query_id"),
			BlockedQuery:     ptr("blocked_query"),
			BlockedStatus:    ptr("blocked_status"),
			BlockedHost:      ptr("blocked_host"),
			BlockedDB:        ptr("blocked_db"),
			BlockingTxnID:    ptr("blocking_txn_id"),
			BlockingPID:      ptr("blocking_pid"),
			BlockingThreadID: ptr(int64(456)),
			BlockingStatus:   ptr("blocking_status"),
			BlockingHost:     ptr("blocking_host"),
			BlockingQueryID:  ptr("blocking_query_id"),
			BlockingQuery:    ptr("blocking_query"),
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
