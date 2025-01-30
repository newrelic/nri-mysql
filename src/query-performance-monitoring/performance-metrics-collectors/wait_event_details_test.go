package performancemetricscollectors

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	utils "github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func convertNullString(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func convertToDriverValue(args []interface{}) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, v := range args {
		values[i] = driver.Value(v)
	}
	return values
}

func (m *MockIntegration) IngestMetric(metricList []interface{}, eventType string, i *integration.Integration, args args.ArgumentList) error {
	argsMock := m.Called(metricList, eventType, i, args)
	return argsMock.Error(0)
}

type DataSource struct {
	DB *sqlx.DB
}

func (ds *DataSource) QueryX(query string) (*sqlx.Rows, error) {
	return ds.DB.Queryx(query)
}

func (ds *DataSource) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return ds.DB.QueryxContext(ctx, query, args...)
}

func (ds *DataSource) Close() {
	ds.DB.Close()
}

func convertNullFloat64(ns sql.NullFloat64) *float64 {
	if ns.Valid {
		return &ns.Float64
	}
	return nil
}

func TestPopulateWaitEventMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	dataSource := &DataSource{DB: sqlxDB}
	i, err := integration.New("test-integration", "1.0.0")
	require.NoError(t, err)
	args := args.ArgumentList{QueryMonitoringCountThreshold: 10, QueryMonitoringResponseTimeThreshold: 10}
	excludedDatabases := []string{"mysql", "information_schema"}

	// Prepare the arguments for the query
	excludedDatabasesArgs := []interface{}{excludedDatabases, excludedDatabases, min(args.QueryMonitoringCountThreshold, constants.MaxQueryCountThreshold)}

	// Prepare the SQL query with the provided parameters
	preparedQuery, preparedArgs, err := sqlx.In(utils.WaitEventsQuery, excludedDatabasesArgs...)
	require.NoError(t, err)

	// Rebind the query for the sqlmock driver
	preparedQuery = sqlx.Rebind(sqlx.QUESTION, preparedQuery)

	// Mock the query execution
	mock.ExpectQuery(regexp.QuoteMeta(preparedQuery)).WithArgs(convertToDriverValue(preparedArgs)...).WillReturnRows(sqlmock.NewRows([]string{
		"wait_event_name", "wait_category", "total_wait_time_ms", "collection_timestamp", "query_id", "query_text", "database_name",
	}).AddRow(
		"Locks:Lock", "Locks", 1000.0, "2023-01-01T00:00:00Z", "queryid1", "SELECT 1", "testdb",
	))

	// Call the function under test
	PopulateWaitEventMetrics(dataSource, i, args, excludedDatabases)
	assert.NoError(t, err)

	// Verify that all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSetWaitEventMetrics tests the setWaitEventMetrics function.
func TestSetWaitEventQueryMetrics(t *testing.T) {
	i, err := integration.New("test", "1.0.0")
	require.NoError(t, err)
	e := i.LocalEntity()
	args := args.ArgumentList{}
	metrics := []utils.WaitEventQueryMetrics{
		{
			WaitEventName:       convertNullString(sql.NullString{String: "wait_event_name", Valid: true}),
			WaitCategory:        convertNullString(sql.NullString{String: "wait_category", Valid: true}),
			TotalWaitTimeMs:     convertNullFloat64(sql.NullFloat64{Float64: 1000.0, Valid: true}),
			CollectionTimestamp: convertNullString(sql.NullString{String: "2023-01-01T00:00:00Z", Valid: true}),
			QueryID:             convertNullString(sql.NullString{String: "queryid1", Valid: true}),
			QueryText:           convertNullString(sql.NullString{String: "SELECT 1", Valid: true}),
			DatabaseName:        convertNullString(sql.NullString{String: "testdb", Valid: true}),
		},
	}
	err = setWaitEventMetrics(i, args, metrics)
	assert.NoError(t, err)
	ms := e.Metrics[0]
	assert.Equal(t, "wait_event_name", ms.Metrics["wait_event_name"])
	assert.Equal(t, "wait_category", ms.Metrics["wait_category"])
	assert.Equal(t, float64(1000.0), ms.Metrics["total_wait_time_ms"])
	assert.Equal(t, "2023-01-01T00:00:00Z", ms.Metrics["collection_timestamp"])
	assert.Equal(t, "queryid1", ms.Metrics["query_id"])
	assert.Equal(t, "SELECT 1", ms.Metrics["query_text"])
	assert.Equal(t, "testdb", ms.Metrics["database_name"])
}
