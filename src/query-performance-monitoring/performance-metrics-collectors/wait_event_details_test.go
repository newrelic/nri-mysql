package performancemetricscollectors

import (
	"context"
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

func (m *MockIntegration) IngestMetric(metricList []interface{}, eventType string, i *integration.Integration, args args.ArgumentList) error {
	argsMock := m.Called(metricList, eventType, i, args)
	return argsMock.Error(0)
}

func convertToDriverValue(args []interface{}) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, v := range args {
		values[i] = driver.Value(v)
	}
	return values
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

func TestPopulateWaitEventMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	dataSource := &DataSource{DB: sqlxDB}
	i, err := integration.New("test-integration", "1.0.0")
	require.NoError(t, err)
	e := i.LocalEntity()
	args := args.ArgumentList{QueryCountThreshold: 10, QueryResponseTimeThreshold: 10}
	excludedDatabases := []string{"mysql", "information_schema"}

	// Prepare the arguments for the query
	excludedDatabasesArgs := []interface{}{excludedDatabases, excludedDatabases, min(args.QueryCountThreshold, constants.MaxQueryCountThreshold)}

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
	PopulateWaitEventMetrics(dataSource, i, e, args, excludedDatabases)
	assert.NoError(t, err)

	// Verify that all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSetWaitEventMetrics tests the setWaitEventMetrics function.
func TestSetWaitEventMetrics(t *testing.T) {
	i, err := integration.New("test-integration", "1.0.0")
	require.NoError(t, err)

	args := args.ArgumentList{}
	metrics := []utils.WaitEventQueryMetrics{
		{
			QueryID:             StringPtr("digest1"),
			InstanceID:          StringPtr("instance1"),
			DatabaseName:        StringPtr("testdb"),
			WaitEventName:       StringPtr("wait/io/file/innodb/log"),
			WaitCategory:        StringPtr("InnoDB File IO"),
			TotalWaitTimeMs:     Float64Ptr(1.0),
			WaitEventCount:      Uint64Ptr(1),
			AvgWaitTimeMs:       StringPtr("1.0"),
			QueryText:           StringPtr("SELECT * FROM table1"),
			CollectionTimestamp: StringPtr("2023-10-01T12:00:00Z"),
		},
	}

	err = setWaitEventMetrics(i, args, metrics)
	assert.NoError(t, err)
}
