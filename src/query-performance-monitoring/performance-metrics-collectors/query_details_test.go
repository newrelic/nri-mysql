package performancemetricscollectors

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func NewMockDataSource(db *sqlx.DB) *MockDataSource {
	return &MockDataSource{db: db}
}

func (m *MockDataSource) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	arguments := m.Called(query, args)
	return arguments.Get(0).([]map[string]interface{}), arguments.Error(1)
}

func stringPtr(s string) *string {
	return &s
}

var (
	mockCollectIndividualQueryMetrics  func(db utils.DataSource, queryIDList []string, searchType string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error)
	mockCollectGroupedSlowQueryMetrics func(db utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error)
	mockSetSlowQueryMetrics            func(i *integration.Integration, rawMetrics []map[string]interface{}, args arguments.ArgumentList) error
)

var (
	errSomeError              = errors.New("some error")
	errFailedToCollectMetrics = errors.New("failed to collect metrics")
	errFailedToSetMetrics     = errors.New("failed to set metrics")
)

func TestSetSlowQueryMetrics(t *testing.T) {
	mockIntegration := new(MockIntegration)
	mockIntegration.Integration, _ = integration.New("test", "1.0.0") // Properly initialize the Integration field
	mockArgs := arguments.ArgumentList{}

	t.Run("Successful Ingestion", func(t *testing.T) {
		metrics := []utils.SlowQueryMetrics{
			{QueryID: stringPtr("1"), QueryText: stringPtr("SELECT * FROM table1")},
		}
		mockIntegration.On("IngestMetric", mock.Anything, "MysqlSlowQuerySample", mockIntegration, mockArgs).Return(nil)

		err := setSlowQueryMetrics(mockIntegration.Integration, metrics, mockArgs)
		assert.NoError(t, err)
	})

	t.Run("Empty Metrics", func(t *testing.T) {
		metrics := []utils.SlowQueryMetrics{}
		err := setSlowQueryMetrics(mockIntegration.Integration, metrics, mockArgs)
		assert.NoError(t, err)
	})
}

func TestGroupQueriesByDatabase(t *testing.T) {
	tests := []struct {
		name           string
		filteredList   []utils.IndividualQueryMetrics
		expectedGroups []utils.QueryGroup
	}{
		{
			name: "Group queries by database",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table1")},
				{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table2")},
				{DatabaseName: stringPtr("db2"), QueryText: stringPtr("SELECT * FROM table3")},
			},
			expectedGroups: []utils.QueryGroup{
				{
					Database: "db1",
					Queries: []utils.IndividualQueryMetrics{
						{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table1")},
						{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table2")},
					},
				},
				{
					Database: "db2",
					Queries: []utils.IndividualQueryMetrics{
						{DatabaseName: stringPtr("db2"), QueryText: stringPtr("SELECT * FROM table3")},
					},
				},
			},
		},
		{
			name: "Handle nil database name",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: nil, QueryText: stringPtr("SELECT * FROM table1")},
				{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table2")},
			},
			expectedGroups: []utils.QueryGroup{
				{
					Database: "db1",
					Queries: []utils.IndividualQueryMetrics{
						{DatabaseName: stringPtr("db1"), QueryText: stringPtr("SELECT * FROM table2")},
					},
				},
			},
		},
		{
			name:           "Empty filtered list",
			filteredList:   []utils.IndividualQueryMetrics{},
			expectedGroups: []utils.QueryGroup{},
		},
		{
			name: "All nil database names",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: nil, QueryText: stringPtr("SELECT * FROM table1")},
				{DatabaseName: nil, QueryText: stringPtr("SELECT * FROM table2")},
			},
			expectedGroups: []utils.QueryGroup{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualGroups := groupQueriesByDatabase(tt.filteredList)
			assert.ElementsMatch(t, tt.expectedGroups, actualGroups)
		})
	}
}

func TestCollectIndividualQueryMetrics(t *testing.T) {
	mockDB := new(MockDataSource)
	queryIDList := []string{"1", "2", "3"}
	queryString := "SELECT * FROM performance_schema.events_statements_current WHERE DIGEST = ? AND TIMER_WAIT / 1000000000 > ? ORDER BY TIMER_WAIT DESC LIMIT ?"
	args := arguments.ArgumentList{
		QueryResponseTimeThreshold: 1,
		QueryCountThreshold:        10,
	}

	runCollectIndividualQueryMetricsTest := func(name string, queryIDList []string, expectedError error, expectedMetrics []utils.IndividualQueryMetrics) {
		t.Run(name, func(t *testing.T) {
			mockCollectIndividualQueryMetrics = func(_ utils.DataSource, _ []string, _ string, args arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
				return expectedMetrics, expectedError
			}

			rows := sqlx.Rows{}
			mockDB.On("QueryxContext", mock.Anything, mock.Anything, mock.Anything).Return(&rows, expectedError)

			actualMetrics, err := collectIndividualQueryMetrics(mockDB, queryIDList, queryString, args)
			if expectedError != nil {
				assert.Error(t, err)
				assert.NotNil(t, actualMetrics)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedMetrics, actualMetrics)
			}
		})
	}

	runCollectIndividualQueryMetricsTest("Error", queryIDList, errSomeError, nil)
	runCollectIndividualQueryMetricsTest("EmptyQueryIDList", []string{}, nil, []utils.IndividualQueryMetrics{})
}

func TestExtensiveQueryMetrics(t *testing.T) {
	mockDB := new(MockDataSource)
	queryIDList := []string{"1", "2", "3"}
	args := arguments.ArgumentList{}

	runExtensiveQueryMetricsTest := func(name string, queryIDList []string, expectedError error, expectedMetrics []utils.IndividualQueryMetrics) {
		t.Run(name, func(t *testing.T) {
			mockCollectIndividualQueryMetrics = func(_ utils.DataSource, _ []string, _ string, _ arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
				return expectedMetrics, expectedError
			}

			rows := sqlx.Rows{}
			mockDB.On("QueryxContext", mock.Anything, mock.Anything, mock.Anything).Return(&rows, expectedError)

			actualMetrics, err := extensiveQueryMetrics(mockDB, queryIDList, args)
			if expectedError != nil {
				assert.Error(t, err)
				assert.Nil(t, actualMetrics)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedMetrics, actualMetrics)
			}
		})
	}

	runExtensiveQueryMetricsTest("Error", queryIDList, errSomeError, nil)
	runExtensiveQueryMetricsTest("EmptyQueryIDList", []string{}, nil, []utils.IndividualQueryMetrics{})
}

func TestPopulateSlowQueryMetrics(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := sqlx.NewDb(sqlDB, "sqlmock")
	defer db.Close()

	mockDB := NewMockDataSource(db)
	mockIntegration := new(MockIntegration)
	mockIntegration.Integration, _ = integration.New("test", "1.0.0")
	args := arguments.ArgumentList{
		SlowQueryFetchInterval: 60,
		QueryCountThreshold:    10,
	}
	excludedDatabases := []string{}

	t.Run("Failure to collect slow query metrics", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return nil, nil, errFailedToCollectMetrics
		}

		queryIDList := PopulateSlowQueryMetrics(mockIntegration.Integration, mockDB, args, excludedDatabases)
		assert.Empty(t, queryIDList)
	})

	t.Run("No metrics collected", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return []utils.IndividualQueryMetrics{}, []string{}, nil
		}

		queryIDList := PopulateSlowQueryMetrics(mockIntegration.Integration, mockDB, args, excludedDatabases)
		assert.Empty(t, queryIDList)
	})

	t.Run("Failure to set slow query metrics", func(t *testing.T) {
		expectedMetrics := []map[string]interface{}{
			{"query_id": "1", "query_text": "SELECT * FROM table1"},
			{"query_id": "2", "query_text": "SELECT * FROM table2"},
		}
		expectedQueryIDList := []string{"1", "2"}

		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, _ int, _ int, _ []string) ([]utils.IndividualQueryMetrics, []string, error) {
			metrics := []utils.IndividualQueryMetrics{}
			for _, m := range expectedMetrics {
				metrics = append(metrics, utils.IndividualQueryMetrics{
					QueryID:   stringPtr(m["query_id"].(string)),
					QueryText: stringPtr(m["query_text"].(string)),
				})
			}
			return metrics, expectedQueryIDList, nil
		}

		mockSetSlowQueryMetrics = func(_ *integration.Integration, _ []map[string]interface{}, _ arguments.ArgumentList) error {
			return errFailedToSetMetrics
		}

		queryIDList := PopulateSlowQueryMetrics(mockIntegration.Integration, mockDB, args, excludedDatabases)
		assert.Empty(t, queryIDList)
	})
}
