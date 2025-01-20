package performancemetricscollectors

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"

	arguments "github.com/newrelic/nri-mysql/src/args"
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

func TestPopulateBlockingSessionMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockRows := sqlmock.NewRows([]string{})

	query := utils.BlockingSessionsQuery

	// Define the arguments
	excludedDatabases := []string{"", "mysql", "information_schema", "performance_schema", "sys"}
	queryCountThreshold := 10

	// Use sqlx.In to bind the arguments
	query, args, err := sqlx.In(query, excludedDatabases, queryCountThreshold)
	if err != nil {
		t.Fatalf("failed to bind query arguments: %v", err)
	}

	driverArgs := make([]driver.Value, len(args))
	for i, arg := range args {
		driverArgs[i] = driver.Value(arg)
	}
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(driverArgs...).WillReturnRows(mockRows)

	dataSource := &dbWrapper{DB: sqlx.NewDb(db, "sqlmock")}
	i, _ := integration.New("test", "1.0.0")
	// Convert []string to string
	excludedDatabasesStr := strings.Join(excludedDatabases, ",")
	argList := arguments.ArgumentList{ExcludedPerformanceDatabases: excludedDatabasesStr, QueryCountThreshold: queryCountThreshold}

	PopulateBlockingSessionMetrics(dataSource, i, argList, []string{})
	assert.NoError(t, err)
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
