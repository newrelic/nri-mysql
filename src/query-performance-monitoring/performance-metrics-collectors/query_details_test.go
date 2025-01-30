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
	selectQuery := "SELECT * FROM users"
	updateQuery := "UPDATE users SET name = 'test' WHERE id = 1"
	queryID := "1"
	i, err := integration.New("test-integration", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}

	metrics := []utils.SlowQueryMetrics{
		{QueryText: &selectQuery, QueryID: &queryID},
		{QueryText: &updateQuery},
	}
	args := arguments.ArgumentList{}

	err = setSlowQueryMetrics(i, metrics, args)
	assert.NoError(t, err)
}

func TestGroupQueriesByDatabase(t *testing.T) {
	queryText1 := "SELECT * FROM test_table1"
	queryText2 := "SELECT * FROM test_table2"
	queryText3 := "SELECT * FROM test_table3"
	database1 := "db1"
	database2 := "db2"
	tests := []struct {
		name           string
		filteredList   []utils.IndividualQueryMetrics
		expectedGroups map[string][]utils.IndividualQueryMetrics
	}{
		{
			name: "Group queries by database",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: &database1, QueryText: &queryText1},
				{DatabaseName: &database1, QueryText: &queryText2},
				{DatabaseName: &database2, QueryText: &queryText3},
			},
			expectedGroups: map[string][]utils.IndividualQueryMetrics{
				database1: {
					{DatabaseName: &database1, QueryText: &queryText1},
					{DatabaseName: &database1, QueryText: &queryText2},
				},
				database2: {
					{DatabaseName: &database2, QueryText: &queryText3},
				},
			},
		},
		{
			name: "Handle nil database name",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: nil, QueryText: &queryText1},
				{DatabaseName: &database1, QueryText: &queryText2},
			},
			expectedGroups: map[string][]utils.IndividualQueryMetrics{
				database1: {
					{DatabaseName: &database1, QueryText: &queryText2},
				},
			},
		},
		{
			name:           "Empty filtered list",
			filteredList:   []utils.IndividualQueryMetrics{},
			expectedGroups: map[string][]utils.IndividualQueryMetrics{},
		},
		{
			name: "All nil database names",
			filteredList: []utils.IndividualQueryMetrics{
				{DatabaseName: nil, QueryText: &queryText1},
				{DatabaseName: nil, QueryText: &queryText2},
			},
			expectedGroups: map[string][]utils.IndividualQueryMetrics{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualGroups := groupQueriesByDatabase(tt.filteredList)
			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// Helper function to handle assertions
func assertQueryMetrics(t *testing.T, actualMetrics []utils.IndividualQueryMetrics, err error, expectedError error, expectedMetrics []utils.IndividualQueryMetrics) {
	if expectedError != nil {
		assert.Error(t, err)
		assert.NotNil(t, actualMetrics)
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedMetrics, actualMetrics)
	}
}

func TestCollectIndividualQueryMetrics(t *testing.T) {
	mockDB := new(MockDataSource)
	args := arguments.ArgumentList{
		QueryMonitoringResponseTimeThreshold: 1,
		QueryMonitoringCountThreshold:        10,
	}

	tests := []struct {
		name            string
		queryIDList     []string
		expectedError   error
		expectedMetrics []utils.IndividualQueryMetrics
	}{
		{
			name:            "Error",
			queryIDList:     []string{"1", "2", "3"},
			expectedError:   errSomeError,
			expectedMetrics: nil,
		},
		{
			name:            "EmptyQueryIDList",
			queryIDList:     []string{},
			expectedError:   nil,
			expectedMetrics: []utils.IndividualQueryMetrics{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollectIndividualQueryMetrics = func(_ utils.DataSource, _ []string, _ string, _ arguments.ArgumentList) ([]utils.IndividualQueryMetrics, error) {
				return tt.expectedMetrics, tt.expectedError
			}

			rows := sqlx.Rows{}
			mockDB.On("QueryxContext", mock.Anything, mock.Anything, mock.Anything).Return(&rows, tt.expectedError)

			actualMetrics, err := collectIndividualQueryMetrics(mockDB, tt.queryIDList, utils.CurrentRunningQueriesSearch, args)
			assertQueryMetrics(t, actualMetrics, err, tt.expectedError, tt.expectedMetrics)
		})
	}
}

func TestPopulateSlowQueryMetrics(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := sqlx.NewDb(sqlDB, "sqlmock")
	defer db.Close()

	mockDB := NewMockDataSource(db)
	i, err := integration.New("test-integration", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}
	args := arguments.ArgumentList{
		SlowQueryFetchInterval:        60,
		QueryMonitoringCountThreshold: 10,
	}
	excludedDatabases := []string{}

	t.Run("Failure to collect slow query metrics", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return nil, nil, errFailedToCollectMetrics
		}

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases)
		assert.Empty(t, queryIDList)
	})

	t.Run("No metrics collected", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return []utils.IndividualQueryMetrics{}, []string{}, nil
		}

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases)
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
				queryID := m["query_id"].(string)
				queryText := m["query_text"].(string)
				metrics = append(metrics, utils.IndividualQueryMetrics{
					QueryID:   &queryID,
					QueryText: &queryText,
				})
			}
			return metrics, expectedQueryIDList, nil
		}

		mockSetSlowQueryMetrics = func(_ *integration.Integration, _ []map[string]interface{}, _ arguments.ArgumentList) error {
			return errFailedToSetMetrics
		}

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases)
		assert.Empty(t, queryIDList)
	})
}
