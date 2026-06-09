package utils

import (
	"encoding/json"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"

	"github.com/stretchr/testify/assert"
)

func TestSetMetric(t *testing.T) {
	i, _ := integration.New("test", "1.0.0")
	entity := i.LocalEntity()
	metricSet := entity.NewMetricSet("testEvent")

	tests := []struct {
		name       string
		metricName string
		value      interface{}
		sourceType string
	}{
		{"GaugeMetric", "gaugeMetric", float64(123), "gauge"},
		{"AttributeMetric", "attributeMetric", "value", "attribute"},
		{"DefaultMetric", "defaultMetric", float64(456), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMetric(metricSet, tt.metricName, tt.value, tt.sourceType)
			metricValue, ok := metricSet.Metrics[tt.metricName]
			assert.True(t, ok)
			assert.Equal(t, tt.value, metricValue)
		})
	}
}

func TestConvertToInterfaceSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []interface{}
	}{
		{
			name:     "EmptySlice",
			input:    []string{},
			expected: []interface{}{},
		},
		{
			name:     "SingleElement",
			input:    []string{"one"},
			expected: []interface{}{"one"},
		},
		{
			name:     "MultipleElements",
			input:    []string{"one", "two", "three"},
			expected: []interface{}{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToInterfaceSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessModel(t *testing.T) {
	type TestModel struct {
		Field1 string  `metric_name:"field1_metric" source_type:"attribute"`
		Field2 int     `metric_name:"field2_metric" source_type:"gauge"`
		Field3 *string `metric_name:"field3_metric" source_type:"attribute"`
	}

	i, _ := integration.New("test", "1.0.0")
	entity := i.LocalEntity()

	t.Run("ValidModelWithNonPointerFields", func(t *testing.T) {
		model := TestModel{
			Field1: "value1",
			Field2: 123,
		}
		err := processModel(model, entity, "testEvent", arguments.ArgumentList{})
		assert.NoError(t, err)
	})

	t.Run("ValidModelWithPointerFields", func(t *testing.T) {
		field3Value := "value3"
		model := TestModel{
			Field1: "value1",
			Field2: 123,
			Field3: &field3Value,
		}
		err := processModel(model, entity, "testEvent", arguments.ArgumentList{})
		assert.NoError(t, err)
	})

	t.Run("InvalidModelNotStruct", func(t *testing.T) {
		model := "invalid model"
		err := processModel(model, entity, "testEvent", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Equal(t, ErrModelIsNotValid, err)
	})

	t.Run("InvalidModelNilPointer", func(t *testing.T) {
		var model *TestModel
		err := processModel(model, entity, "testEvent", arguments.ArgumentList{})
		assert.Error(t, err)
		assert.Equal(t, ErrModelIsNotValid, err)
	})
}

func TestIngestMetric(t *testing.T) {
	i, _ := integration.New("test", "1.0.0")

	t.Run("SuccessfulIngestion", func(t *testing.T) {
		metricList := []interface{}{
			struct{}{},
			struct{}{},
		}
		args := arguments.ArgumentList{}
		err := IngestMetric(metricList, "testEvent", i, args)
		assert.NoError(t, err)
	})

	t.Run("NilModelsInList", func(t *testing.T) {
		metricList := []interface{}{
			nil,
			struct{}{},
		}
		args := arguments.ArgumentList{}
		err := IngestMetric(metricList, "testEvent", i, args)
		assert.NoError(t, err)
	})

	t.Run("MetricCountExceedsLimit", func(t *testing.T) {
		metricList := make([]interface{}, constants.MetricSetLimit+1)
		for i := range metricList {
			metricList[i] = struct{}{}
		}
		args := arguments.ArgumentList{}
		err := IngestMetric(metricList, "testEvent", i, args)
		assert.NoError(t, err)
	})
}

func TestAnonymizeQueryText(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		input    *string
		expected *string
	}{
		{
			name:     "NilInput",
			input:    nil,
			expected: nil,
		},
		{
			name:     "AlreadyAnonymized",
			input:    strPtr("SELECT * FROM users WHERE id = ? AND name = ?"),
			expected: strPtr("SELECT * FROM users WHERE id = ? AND name = ?"),
		},
		{
			name:     "StringLiterals",
			input:    strPtr("INSERT INTO t VALUES ('hello', 'world')"),
			expected: strPtr("INSERT INTO t VALUES (?, ?)"),
		},
		{
			name:     "NumericLiterals",
			input:    strPtr("SELECT * FROM orders WHERE id = 42 AND status = 1"),
			expected: strPtr("SELECT * FROM orders WHERE id = ? AND status = ?"),
		},
		{
			name:     "HexLiterals",
			input:    strPtr("SELECT * FROM t WHERE col = 0xDEADBEEF"),
			expected: strPtr("SELECT * FROM t WHERE col = ?"),
		},
		{
			name:     "DecimalLiterals",
			input:    strPtr("SELECT * FROM orders WHERE price > 3.14 AND tax = 0.05"),
			expected: strPtr("SELECT * FROM orders WHERE price > ? AND tax = ?"),
		},
		{
			name:     "MixedLiterals",
			input:    strPtr("UPDATE users SET name = 'Alice' WHERE id = 99 AND token = 0xFF"),
			expected: strPtr("UPDATE users SET name = ? WHERE id = ? AND token = ?"),
		},
		{
			name:     "EmptyString",
			input:    strPtr(""),
			expected: strPtr(""),
		},
		{
			name:     "ColumnIdentifiersWithNumbers",
			input:    strPtr("SELECT col_1, total_2 FROM t WHERE row_id = col_1"),
			expected: strPtr("SELECT col_1, total_2 FROM t WHERE row_id = col_1"),
		},
		{
			name:     "EscapedQuotesInsideString",
			input:    strPtr("SELECT * FROM t WHERE name = 'it''s here' AND city = 'O''Brien'"),
			expected: strPtr("SELECT * FROM t WHERE name = ? AND city = ?"),
		},
		{
			name:     "BackslashEscapeInsideString",
			input:    strPtr(`SELECT * FROM t WHERE name = 'O\'Brien' AND code = 'foo\\bar'`),
			expected: strPtr("SELECT * FROM t WHERE name = ? AND code = ?"),
		},
		{
			name:     "MixedEscapeStyles",
			input:    strPtr(`UPDATE t SET a = 'it''s' WHERE b = 'O\'Brien'`),
			expected: strPtr("UPDATE t SET a = ? WHERE b = ?"),
		},
		{
			name:     "MariaDBBlockingQuery",
			input:    strPtr("UPDATE blocking_test SET balance = balance + 3 WHERE account_id = 1001"),
			expected: strPtr("UPDATE blocking_test SET balance = balance + ? WHERE account_id = ?"),
		},
		{
			name:     "SleepQuery",
			input:    strPtr("SELECT SLEEP(90)"),
			expected: strPtr("SELECT SLEEP(?)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeQueryText(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestGetExcludedDatabases(t *testing.T) {
	type testCase struct {
		Name              string   `json:"name"`
		ExcludedDBList    string   `json:"excludedDBList"`
		ExpectedDatabases []string `json:"expectedDatabases"`
	}

	jsonInput := `[
        {
            "name": "Valid JSON with multiple databases",
            "excludedDBList": "[\"db1\",\"db2\"]",
            "expectedDatabases": ["", "mysql", "information_schema", "performance_schema", "sys", "db1", "db2"]
        },
        {
            "name": "Valid JSON with single database",
            "excludedDBList": "[\"db1\"]",
            "expectedDatabases": ["", "mysql", "information_schema", "performance_schema", "sys", "db1"]
        },
        {
            "name": "Invalid JSON",
            "excludedDBList": "[\"db1\",\"db2\"",
            "expectedDatabases": ["", "mysql", "information_schema", "performance_schema", "sys"]
        },
        {
            "name": "Empty JSON array",
            "excludedDBList": "[]",
            "expectedDatabases": ["", "mysql", "information_schema", "performance_schema", "sys"]
        },
        {
            "name": "Empty string",
            "excludedDBList": "",
            "expectedDatabases": ["", "mysql", "information_schema", "performance_schema", "sys"]
        }
    ]`

	var testCases []testCase
	err := json.Unmarshal([]byte(jsonInput), &testCases)
	assert.NoError(t, err, "Failed to unmarshal JSON input")

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			result := GetExcludedDatabases(tt.ExcludedDBList)
			assert.ElementsMatch(t, tt.ExpectedDatabases, result)
		})
	}
}
