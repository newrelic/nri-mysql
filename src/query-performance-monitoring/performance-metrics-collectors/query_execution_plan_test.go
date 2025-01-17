package performancemetricscollectors

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"

	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock DataSource
type MockDataSource struct {
	mock.Mock
	db *sqlx.DB
}

// QueryX is a mock implementation of the QueryX method.
func (m *MockDataSource) QueryX(query string) (*sqlx.Rows, error) {
	calledArgs := m.Called(query)
	return calledArgs.Get(0).(*sqlx.Rows), calledArgs.Error(1)
}

// Close is a mock implementation of the Close method.
func (m *MockDataSource) Close() {
	// No-op
}

// Uint64Ptr returns a pointer to the uint64 value passed in.

func Uint64Ptr(i uint64) *uint64 {
	return &i
}

// StringPtr returns a pointer to the string value passed in.

func StringPtr(s string) *string {
	return &s
}

// QueryxContext is a mock implementation of the QueryxContext method.
func (m *MockDataSource) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	calledArgs := m.Called(ctx, query, args)
	return calledArgs.Get(0).(*sqlx.Rows), calledArgs.Error(1)
}

// MockUtils is a mock implementation of the Utils interface.
type MockUtils struct {
	mock.Mock
}

// MockDB is a mock implementation of a database connection.
type MockDB struct {
	mock.Mock
}
type Query struct {
	SQL string
}

// MockIntegration is a mock implementation of the Integration interface.
type MockIntegration struct {
	mock.Mock
	*integration.Integration
}

func TestExtractMetricsFromJSONString(t *testing.T) {
	t.Run("valid JSON input", func(t *testing.T) {
		jsonString := `{"table_name": "test_table", "cost_info": {"query_cost": "10"}}`
		eventID := uint64(1)
		threadID := uint64(1)

		metrics, err := extractMetricsFromJSONString(jsonString, eventID, threadID)

		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, "test_table", metrics[0].TableName)
		assert.Equal(t, "10", metrics[0].QueryCost)
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		invalidJSONString := `{"table_name": "test_table", "cost_info": {"query_cost": "10"`
		eventID := uint64(1)
		threadID := uint64(1)

		metrics, err := extractMetricsFromJSONString(invalidJSONString, eventID, threadID)

		assert.Error(t, err)
		assert.Empty(t, metrics)
	})
}

func TestSetExecutionPlanMetrics(t *testing.T) {
	mockIntegration := new(MockIntegration)
	mockIntegration.Integration, _ = integration.New("test", "1.0.0") // Properly initialize the Integration field
	mockArgs := arguments.ArgumentList{}

	t.Run("Successful Ingestion", func(t *testing.T) {
		metrics := []utils.QueryPlanMetrics{
			{EventID: 1, QueryCost: "10", TableName: "test"},
		}
		mockIntegration.On("IngestMetric", mock.Anything, "MysqlQueryExecutionSample", mockIntegration, mockArgs).Return(nil)

		err := SetExecutionPlanMetrics(mockIntegration.Integration, mockArgs, metrics)
		assert.NoError(t, err)
	})

	t.Run("Empty Metrics", func(t *testing.T) {
		metrics := []utils.QueryPlanMetrics{}
		err := SetExecutionPlanMetrics(mockIntegration.Integration, mockArgs, metrics)
		assert.NoError(t, err)
	})
}

func TestIsSupportedStatement(t *testing.T) {
	t.Run("Supported Statement", func(t *testing.T) {
		assert.True(t, isSupportedStatement("SELECT * FROM test"))
		assert.True(t, isSupportedStatement("INSERT INTO test VALUES (1)"))
		assert.True(t, isSupportedStatement("UPDATE test SET value = 1"))
		assert.True(t, isSupportedStatement("DELETE FROM test"))
		assert.True(t, isSupportedStatement("WITH cte AS (SELECT * FROM test) SELECT * FROM cte"))
	})

	t.Run("Unsupported Statement", func(t *testing.T) {
		assert.False(t, isSupportedStatement("DROP TABLE test"))
		assert.False(t, isSupportedStatement("ALTER TABLE test ADD COLUMN value INT"))
	})
}

func TestPopulateExecutionPlans(t *testing.T) {
	tests := []struct {
		name        string
		queryGroups []utils.QueryGroup
		args        arguments.ArgumentList
		setupMocks  func()
		expectError bool
	}{
		{
			name:        "No Queries",
			queryGroups: []utils.QueryGroup{},
			args:        arguments.ArgumentList{},
			setupMocks: func() {
				// No calls to mockUtils, as no queries are provided
			},
			expectError: false,
		},
		{
			name: "Single Query Group",
			queryGroups: []utils.QueryGroup{
				{
					Database: "test_db",
					Queries: []utils.IndividualQueryMetrics{
						{QueryText: StringPtr("SELECT * FROM test_table")},
					},
				},
			},
			args: arguments.ArgumentList{},
			setupMocks: func() {
				mockDB := new(MockDataSource)
				mockDB.On("QueryX", "SELECT * FROM test_table").Return([]utils.QueryPlanMetrics{}, nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tt.setupMocks()
			mockDB := new(MockDataSource)
			mockIntegration := new(integration.Integration)
			mockEntity := new(integration.Entity)

			PopulateExecutionPlans(mockDB, tt.queryGroups, mockIntegration, mockEntity, tt.args)

			mockDB.AssertExpectations(t)
		})
	}
}

func TestProcessSliceValue(t *testing.T) {
	tests := []struct {
		name               string
		value              interface{}
		expectedMetricsLen int
	}{
		{
			name: "Valid JSON Elements",
			value: []interface{}{
				map[string]interface{}{"key1": "value1"},
				map[string]interface{}{"key2": "value2"},
			},
			expectedMetricsLen: 0,
		},
		{
			name:               "Empty Slice",
			value:              []interface{}{},
			expectedMetricsLen: 0,
		},
		{
			name: "Non-map Elements",
			value: []interface{}{
				"string element",
				12345,
			},
			expectedMetricsLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dbPerformanceEvents := make([]utils.QueryPlanMetrics, 0)
			eventID := uint64(1)
			threadID := uint64(1)
			memo := utils.Memo{}
			stepID := new(int)
			*stepID = 1

			// Act
			metrics := processSliceValue(tt.value, dbPerformanceEvents, eventID, threadID, memo, stepID)

			// Debug print
			t.Logf("Metrics: %+v", metrics)

			// Assert
			require.Equal(t, tt.expectedMetricsLen, len(metrics))
		})
	}
}
