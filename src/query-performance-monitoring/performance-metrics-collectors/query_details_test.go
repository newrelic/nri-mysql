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
	assert.NoError(t, err, "Failed to create integration")

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
	assert.NoError(t, err, "Failed to create integration")
	args := arguments.ArgumentList{
		SlowQueryMonitoringFetchInterval: 60,
		QueryMonitoringCountThreshold:    10,
	}
	excludedDatabases := []string{}
	querySet := utils.GetQuerySet(utils.DatabaseFlavorMySQL)

	t.Run("Failure to collect slow query metrics", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return nil, nil, errFailedToCollectMetrics
		}

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases, querySet)
		assert.Empty(t, queryIDList)
	})

	t.Run("No metrics collected", func(t *testing.T) {
		mockCollectGroupedSlowQueryMetrics = func(_ utils.DataSource, fetchInterval int, queryCountThreshold int, excludedDatabases []string) ([]utils.IndividualQueryMetrics, []string, error) {
			return []utils.IndividualQueryMetrics{}, []string{}, nil
		}

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases, querySet)
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

		queryIDList := PopulateSlowQueryMetrics(i, mockDB, args, excludedDatabases, querySet)
		assert.Empty(t, queryIDList)
	})
}

// Helper functions for deduplication tests
func uint64Ptr(value uint64) *uint64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}


// TestDeduplicateIndividualQueryMetrics_EmptySlice tests deduplication with empty input
func TestDeduplicateIndividualQueryMetrics_EmptySlice(t *testing.T) {
	input := []utils.IndividualQueryMetrics{}
	result := deduplicateIndividualQueryMetrics(input)

	assert.Len(t, result, 0, "Expected empty slice")
}




// TestDeduplicateIndividualQueryMetrics_NilEventID tests handling of nil EVENT_ID
func TestDeduplicateIndividualQueryMetrics_NilEventID(t *testing.T) {
	input := []utils.IndividualQueryMetrics{
		{
			EventID:         nil, // Nil EVENT_ID
			ThreadID:        uint64Ptr(50),
			QueryID:         stringPtr("invalid-query-1"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(1.0),
		},
		{
			EventID:         uint64Ptr(500),
			ThreadID:        uint64Ptr(51),
			QueryID:         stringPtr("valid-query"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(2.0),
		},
		{
			EventID:         nil, // Another nil EVENT_ID
			ThreadID:        uint64Ptr(52),
			QueryID:         stringPtr("invalid-query-2"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(3.0),
		},
	}

	result := deduplicateIndividualQueryMetrics(input)

	// Only the valid metric should remain
	assert.Len(t, result, 1, "Expected 1 valid item")
	assert.NotNil(t, result[0].EventID, "Expected non-nil EventID")
	assert.Equal(t, uint64(500), *result[0].EventID, "Expected EventID 500")
}

// TestDeduplicateIndividualQueryMetrics_NilThreadID tests handling of nil THREAD_ID
func TestDeduplicateIndividualQueryMetrics_NilThreadID(t *testing.T) {
	input := []utils.IndividualQueryMetrics{
		{
			EventID:         uint64Ptr(600),
			ThreadID:        nil, // Nil THREAD_ID
			QueryID:         stringPtr("invalid-query-1"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(1.0),
		},
		{
			EventID:         uint64Ptr(601),
			ThreadID:        uint64Ptr(60),
			QueryID:         stringPtr("valid-query"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(2.0),
		},
		{
			EventID:         uint64Ptr(602),
			ThreadID:        nil, // Another nil THREAD_ID
			QueryID:         stringPtr("invalid-query-2"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(3.0),
		},
	}

	result := deduplicateIndividualQueryMetrics(input)

	// Only the valid metric should remain
	assert.Len(t, result, 1, "Expected 1 valid item")
	assert.NotNil(t, result[0].ThreadID, "Expected non-nil ThreadID")
	assert.Equal(t, uint64(60), *result[0].ThreadID, "Expected ThreadID 60")
}

// TestDeduplicateIndividualQueryMetrics_NilExecutionTimeMs tests handling of nil ExecutionTimeMs
func TestDeduplicateIndividualQueryMetrics_NilExecutionTimeMs(t *testing.T) {
	input := []utils.IndividualQueryMetrics{
		{
			EventID:         uint64Ptr(700),
			ThreadID:        uint64Ptr(70),
			QueryID:         stringPtr("invalid-query-1"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: nil, // Nil ExecutionTimeMs
		},
		{
			EventID:         uint64Ptr(701),
			ThreadID:        uint64Ptr(71),
			QueryID:         stringPtr("valid-query"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: float64Ptr(2.0),
		},
		{
			EventID:         uint64Ptr(702),
			ThreadID:        uint64Ptr(72),
			QueryID:         stringPtr("invalid-query-2"),
			DatabaseName:    stringPtr("test_db"),
			ExecutionTimeMs: nil, // Another nil ExecutionTimeMs
		},
	}

	result := deduplicateIndividualQueryMetrics(input)

	// Only the valid metric should remain
	assert.Len(t, result, 1, "Expected 1 valid item")
	assert.NotNil(t, result[0].ExecutionTimeMs, "Expected non-nil ExecutionTimeMs")
	assert.Equal(t, float64(2.0), *result[0].ExecutionTimeMs, "Expected ExecutionTimeMs 2.0")
}

// TestDeduplicateIndividualQueryMetrics_EnhancedLogic tests the enhanced deduplication logic
// that includes execution time to handle multi-application and edge case scenarios
func TestDeduplicateIndividualQueryMetrics_EnhancedLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    []utils.IndividualQueryMetrics
		expected int
		scenario string
	}{
		{
			name: "Standard deduplication - same EVENT_ID, THREAD_ID, different execution times",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(1000),
					ThreadID:        uint64Ptr(45),
					ExecutionTimeMs: float64Ptr(120.500),
				},
				{
					EventID:         uint64Ptr(1000),
					ThreadID:        uint64Ptr(45),
					ExecutionTimeMs: float64Ptr(89.300), // Different execution time
				},
			},
			expected: 2, // Both should be kept (different execution times)
			scenario: "Multi-application edge case - same IDs, different executions",
		},
		{
			name: "True duplicates - identical EVENT_ID, THREAD_ID, execution time",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(2000),
					ThreadID:        uint64Ptr(50),
					ExecutionTimeMs: float64Ptr(45.123),
				},
				{
					EventID:         uint64Ptr(2000),
					ThreadID:        uint64Ptr(50),
					ExecutionTimeMs: float64Ptr(45.123), // Identical - true duplicate
				},
				{
					EventID:         uint64Ptr(2000),
					ThreadID:        uint64Ptr(50),
					ExecutionTimeMs: float64Ptr(45.123), // Identical - true duplicate
				},
			},
			expected: 1, // Only one should be kept (true duplicates filtered)
			scenario: "Performance Schema overlapping tables - same execution",
		},
		{
			name: "Mixed scenario - some duplicates, some unique",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(3000),
					ThreadID:        uint64Ptr(60),
					ExecutionTimeMs: float64Ptr(100.000),
				},
				{
					EventID:         uint64Ptr(3000),
					ThreadID:        uint64Ptr(60),
					ExecutionTimeMs: float64Ptr(100.000), // Duplicate
				},
				{
					EventID:         uint64Ptr(3000),
					ThreadID:        uint64Ptr(60),
					ExecutionTimeMs: float64Ptr(110.500), // Different execution time
				},
				{
					EventID:         uint64Ptr(3001), // Different EVENT_ID
					ThreadID:        uint64Ptr(60),
					ExecutionTimeMs: float64Ptr(100.000),
				},
			},
			expected: 3, // First + third + fourth (second is duplicate of first)
			scenario: "Mixed production scenario",
		},
		{
			name: "Multi-application same query - separate connection pools",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(15001),
					ThreadID:        uint64Ptr(100), // App A connection pool
					ExecutionTimeMs: float64Ptr(45.123),
				},
				{
					EventID:         uint64Ptr(15002),
					ThreadID:        uint64Ptr(200), // App B connection pool
					ExecutionTimeMs: float64Ptr(45.156),
				},
				{
					EventID:         uint64Ptr(15003),
					ThreadID:        uint64Ptr(300), // App C connection pool
					ExecutionTimeMs: float64Ptr(45.089),
				},
			},
			expected: 3, // All should be kept (different connection pools)
			scenario: "Multi-application with separate connection pools",
		},
		{
			name: "Multi-application same query - shared connection pool",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(20001),
					ThreadID:        uint64Ptr(150), // Shared pool thread
					ExecutionTimeMs: float64Ptr(55.200),
				},
				{
					EventID:         uint64Ptr(20002), // Different EVENT_ID
					ThreadID:        uint64Ptr(150), // Same thread (reused)
					ExecutionTimeMs: float64Ptr(55.189),
				},
				{
					EventID:         uint64Ptr(20003), // Different EVENT_ID
					ThreadID:        uint64Ptr(150), // Same thread (reused)
					ExecutionTimeMs: float64Ptr(55.156),
				},
			},
			expected: 3, // All should be kept (different EVENT_IDs)
			scenario: "Multi-application with shared connection pool",
		},
		{
			name: "Edge case - same EVENT_ID, THREAD_ID, similar execution times",
			input: []utils.IndividualQueryMetrics{
				{
					EventID:         uint64Ptr(25000),
					ThreadID:        uint64Ptr(75),
					ExecutionTimeMs: float64Ptr(123.456),
				},
				{
					EventID:         uint64Ptr(25000),
					ThreadID:        uint64Ptr(75),
					ExecutionTimeMs: float64Ptr(123.457), // 1ms difference
				},
			},
			expected: 2, // Both should be kept (different execution times)
			scenario: "Microsecond precision handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateIndividualQueryMetrics(tt.input)
			assert.Equal(t, tt.expected, len(result),
				"Test case: %s\nScenario: %s\nExpected %d metrics, got %d",
				tt.name, tt.scenario, tt.expected, len(result))

			// Basic validation: ensure result length matches expected
			// The actual deduplication logic is tested by checking the expected count
		})
	}
}



