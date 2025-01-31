package performancemetricscollectors

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"

	"github.com/bitly/go-simplejson"
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var openSQLXDB func(dsn string) (*sqlx.DB, error)

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

// QueryxContext is a mock implementation of the QueryxContext method.
func (m *MockDataSource) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	calledArgs := m.Called(ctx, query, args)
	return calledArgs.Get(0).(*sqlx.Rows), calledArgs.Error(1)
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
		jsonString := `{
            "table_name": "test_table",
            "cost_info": {
                "query_cost": "1.23",
                "read_cost": "0.45",
                "eval_cost": "0.12",
                "prefix_cost": "0.66",
                "data_read_per_join": "1024"
            },
            "access_type": "ALL",
            "rows_examined_per_scan": 100,
            "rows_produced_per_join": 50,
            "filtered": "10.00",
            "using_index": true,
            "key_length": "10",
            "possible_keys": ["key1", "key2"],
            "key": "key1",
            "used_key_parts": ["key1_part1", "key1_part2"],
            "ref": ["const"]
        }`
		eventID := uint64(1)
		threadID := uint64(1)

		expectedMetrics := []utils.QueryPlanMetrics{
			{
				EventID:             eventID,
				ThreadID:            threadID,
				TableName:           "test_table",
				QueryCost:           "1.23",
				ReadCost:            "0.45",
				EvalCost:            "0.12",
				PrefixCost:          "0.66",
				DataReadPerJoin:     "1024",
				AccessType:          "ALL",
				RowsExaminedPerScan: 100,
				RowsProducedPerJoin: 50,
				Filtered:            "10.00",
				UsingIndex:          "true",
				KeyLength:           "10",
				PossibleKeys:        "key1,key2",
				Key:                 "key1",
				UsedKeyParts:        "key1_part1,key1_part2",
				Ref:                 "const",
			},
		}

		metrics, err := extractMetricsFromJSONString(jsonString, eventID, threadID)

		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, expectedMetrics, metrics)
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

func getTestCases() []struct {
	name                 string
	jsonString           string
	expectedTableName    string
	expectedQueryCost    string
	expectedAccessType   string
	expectedRowsExamined int64
	eventID              uint64
	threadID             uint64
} {
	return append(
		getSelectQueryTestCases(),
		getWithClauseTestCases()...,
	)
}

func getSelectQueryTestCases() []struct {
	name                 string
	jsonString           string
	expectedTableName    string
	expectedQueryCost    string
	expectedAccessType   string
	expectedRowsExamined int64
	eventID              uint64
	threadID             uint64
} {
	return []struct {
		name                 string
		jsonString           string
		expectedTableName    string
		expectedQueryCost    string
		expectedAccessType   string
		expectedRowsExamined int64
		eventID              uint64
		threadID             uint64
	}{
		{
			name: "SelectQuery_PrimaryKey",
			jsonString: `{
                "table_name": "user_table",
                "cost_info": {
                    "query_cost": "5.0"
                },
                "access_type": "CONST",
                "key": "PRIMARY",
                "rows_examined_per_scan": 1
            }`,
			expectedTableName:    "user_table",
			expectedQueryCost:    "5.0",
			expectedAccessType:   "CONST",
			expectedRowsExamined: 1,
			eventID:              1,
			threadID:             1,
		},
		{
			name: "SelectQuery_Index",
			jsonString: `{
                "table_name": "orders",
                "cost_info": {
                    "query_cost": "12.5"
                },
                "access_type": "ref",
                "key": "idx_order_date",
                "rows_examined_per_scan": 10
            }`,
			expectedTableName:    "orders",
			expectedQueryCost:    "12.5",
			expectedAccessType:   "ref",
			expectedRowsExamined: 10,
			eventID:              2,
			threadID:             2,
		},
		{
			name: "SelectQuery_FullTableScan",
			jsonString: `{
                "table_name": "products",
                "cost_info": {
                    "query_cost": "50.0"
                },
                "access_type": "ALL",
                "rows_examined_per_scan": 1000
            }`,
			expectedTableName:    "products",
			expectedQueryCost:    "50.0",
			expectedAccessType:   "ALL",
			expectedRowsExamined: 1000,
			eventID:              3,
			threadID:             3,
		},
		{
			name: "SelectQuery_WhereClause_IndexRange",
			jsonString: `{
                "table_name": "products",
                "cost_info": {
                    "query_cost": "25.5"
                },
                "access_type": "range",
                "key": "idx_price",
                "rows_examined_per_scan": 200,
                "attached_condition": "price > 100 AND price < 500"
            }`,
			expectedTableName:    "products",
			expectedQueryCost:    "25.5",
			expectedAccessType:   "range",
			expectedRowsExamined: 200,
			eventID:              6,
			threadID:             6,
		},
	}
}

func getWithClauseTestCases() []struct {
	name                 string
	jsonString           string
	expectedTableName    string
	expectedQueryCost    string
	expectedAccessType   string
	expectedRowsExamined int64
	eventID              uint64
	threadID             uint64
} {
	return []struct {
		name                 string
		jsonString           string
		expectedTableName    string
		expectedQueryCost    string
		expectedAccessType   string
		expectedRowsExamined int64
		eventID              uint64
		threadID             uint64
	}{
		{
			name: "WithClause_Materialized",
			jsonString: `{
                "table_name": "temp_table",
                "cost_info": {
                    "query_cost": "2.0"
                },
                "access_type": "ALL", 
                "rows_examined_per_scan": 50
            }`,
			expectedTableName:    "temp_table",
			expectedQueryCost:    "2.0",
			expectedAccessType:   "ALL",
			expectedRowsExamined: 50,
			eventID:              4,
			threadID:             4,
		},
		{
			name: "WithClause_Merged",
			jsonString: `{
                "table_name": "parent_table",
                "cost_info": {
                    "query_cost": "8.5"
                },
                "access_type": "ref",
                "key": "fk_parent_id",
                "rows_examined_per_scan": 5
            }`,
			expectedTableName:    "parent_table",
			expectedQueryCost:    "8.5",
			expectedAccessType:   "ref",
			expectedRowsExamined: 5,
			eventID:              5,
			threadID:             5,
		},
	}
}

func runTestCase(t *testing.T, tc struct {
	name                 string
	jsonString           string
	expectedTableName    string
	expectedQueryCost    string
	expectedAccessType   string
	expectedRowsExamined int64
	eventID              uint64
	threadID             uint64
}) {
	js, err := simplejson.NewJson([]byte(tc.jsonString))
	assert.NoError(t, err)

	memo := utils.Memo{QueryCost: ""}
	stepID := 0
	dbPerformanceEvents := make([]utils.QueryPlanMetrics, 0)

	dbPerformanceEvents = extractMetrics(js, dbPerformanceEvents, tc.eventID, tc.threadID, memo, &stepID)

	assert.Equal(t, 1, len(dbPerformanceEvents))
	assert.Equal(t, tc.expectedTableName, dbPerformanceEvents[0].TableName)
	assert.Equal(t, tc.expectedQueryCost, dbPerformanceEvents[0].QueryCost)
	assert.Equal(t, tc.expectedAccessType, dbPerformanceEvents[0].AccessType)
	assert.Equal(t, tc.expectedRowsExamined, dbPerformanceEvents[0].RowsExaminedPerScan)
}

func TestExtractMetrics_SelectAndWithClause(t *testing.T) {
	testCases := getTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestSetExecutionPlanMetrics(t *testing.T) {
	i, err := integration.New("test-integration", "1.0.0")
	assert.NoError(t, err, "Failed to create integration")
	mockArgs := arguments.ArgumentList{}

	t.Run("Successful Ingestion", func(t *testing.T) {
		metrics := []utils.QueryPlanMetrics{
			{EventID: 1, QueryCost: "10", TableName: "test"},
		}

		err := SetExecutionPlanMetrics(i, mockArgs, metrics)
		assert.NoError(t, err)
	})

	t.Run("Empty Metrics", func(t *testing.T) {
		metrics := []utils.QueryPlanMetrics{}
		err := SetExecutionPlanMetrics(i, mockArgs, metrics)
		assert.NoError(t, err)

		// Verify that no metrics were ingested
		ingestedMetrics := i.Entities[0].Metrics
		assert.Len(t, ingestedMetrics, 0)
	})
}

func TestIsSupportedStatement(t *testing.T) {
	t.Run("Supported Statement", func(t *testing.T) {
		assert.True(t, isSupportedStatement("SELECT * FROM test"))
		assert.True(t, isSupportedStatement("WITH cte AS (SELECT * FROM test) SELECT * FROM cte"))
	})

	t.Run("Unsupported Statement", func(t *testing.T) {
		assert.False(t, isSupportedStatement("DROP TABLE test"))
		assert.False(t, isSupportedStatement("ALTER TABLE test ADD COLUMN value INT"))
		assert.False(t, isSupportedStatement("INSERT INTO test VALUES (1)"))
		assert.False(t, isSupportedStatement("UPDATE test SET value = 1"))
		assert.False(t, isSupportedStatement("DELETE FROM test"))
	})
}

func TestPopulateExecutionPlans(t *testing.T) {
	queryText := "SELECT * FROM test_table"
	mockDB := new(MockDataSource)
	mockIntegration := new(MockIntegration)
	mockIntegration.Integration, _ = integration.New("test", "1.0.0")
	mockArgs := arguments.ArgumentList{}

	queryGroups := map[string][]utils.IndividualQueryMetrics{
		"test_db": {
			{QueryText: &queryText},
		},
	}

	// Mock the OpenSQLXDB function to return the mockDB
	openSQLXDB = func(_ string) (*sqlx.DB, error) {
		return mockDB.db, nil
	}

	t.Run("Error Opening Database Connection", func(t *testing.T) {
		openSQLXDB = func(_ string) (*sqlx.DB, error) {
			return nil, assert.AnError
		}

		PopulateExecutionPlans(mockDB, queryGroups, mockIntegration.Integration, mockArgs)

		mockDB.AssertExpectations(t)
		mockIntegration.AssertExpectations(t)
	})

	t.Run("No Metrics Collected", func(t *testing.T) {
		queryGroups := map[string][]utils.IndividualQueryMetrics{}

		PopulateExecutionPlans(mockDB, queryGroups, mockIntegration.Integration, mockArgs)

		mockDB.AssertExpectations(t)
		mockIntegration.AssertExpectations(t)
	})
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
				map[string]interface{}{"table_name": "table1", "cost_info": map[string]interface{}{"query_cost": "10"}},
				map[string]interface{}{"table_name": "table2", "cost_info": map[string]interface{}{"query_cost": "20"}},
			},
			expectedMetricsLen: 2,
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
		{
			name: "Single Valid Metric",
			value: []interface{}{
				map[string]interface{}{"table_name": "table1", "cost_info": map[string]interface{}{"query_cost": "10"}},
			},
			expectedMetricsLen: 1,
		},
		{
			name: "Multiple Valid Metrics",
			value: []interface{}{
				map[string]interface{}{"table_name": "table1", "cost_info": map[string]interface{}{"query_cost": "10"}},
				map[string]interface{}{"table_name": "table2", "cost_info": map[string]interface{}{"query_cost": "20"}},
			},
			expectedMetricsLen: 2,
		},
		{
			name: "Mixed Valid and Invalid Metrics",
			value: []interface{}{
				map[string]interface{}{"table_name": "table1", "cost_info": map[string]interface{}{"query_cost": "10"}},
				"invalid element",
				map[string]interface{}{"table_name": "table2", "cost_info": map[string]interface{}{"query_cost": "20"}},
			},
			expectedMetricsLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stepID := 0
			metrics := processSliceValue(tt.value, []utils.QueryPlanMetrics{}, 0, 0, utils.Memo{}, &stepID)
			if len(metrics) != tt.expectedMetricsLen {
				assert.Equal(t, tt.expectedMetricsLen, len(metrics), "unexpected metrics length")
			}
		})
	}
}

func TestEscapeAllStringsInJSON_Success(t *testing.T) {
	input := `{"key1": "value1", "key2": "value with \"quotes\" and \\backslashes\\", "key3": ["array", "with", "strings"]}`
	expectedOutput := `{"key1":"value1","key2":"value with \\\"quotes\\\" and \\\\backslashes\\\\","key3":["array","with","strings"]}`

	output, err := escapeAllStringsInJSON(input)
	assert.NoError(t, err, "Expected no error")

	assert.Equal(t, expectedOutput, output, "Output did not match expected output")
}

func TestEscapeAllStringsInJSON_Error(t *testing.T) {
	input := `{"key1": "value1", "key2": "value with "unterminated quote}`

	_, err := escapeAllStringsInJSON(input)
	assert.Error(t, err, "Expected an error")
}
