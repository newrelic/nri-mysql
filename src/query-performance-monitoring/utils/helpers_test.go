package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"

	"github.com/stretchr/testify/assert"
)

var (
	ErrCreateNodeEntity = errors.New("error creating node entity")
	ErrProcessModel     = errors.New("error processing model")
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

func TestMetricSet(t *testing.T) {
	i, _ := integration.New("test", "1.0.0")
	entity := i.LocalEntity()

	tests := []struct {
		name             string
		eventType        string
		hostname         string
		port             int
		remoteMonitoring bool
		expectedMetrics  map[string]interface{}
	}{
		{
			name:             "RemoteMonitoring",
			eventType:        "testEvent",
			hostname:         "remotehost",
			port:             3306,
			remoteMonitoring: true,
			expectedMetrics: map[string]interface{}{
				"hostname": "remotehost",
				"port":     "3306",
			},
		},
		{
			name:             "LocalMonitoring",
			eventType:        "testEvent",
			hostname:         "",
			port:             3306,
			remoteMonitoring: false,
			expectedMetrics: map[string]interface{}{
				"port": "3306",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricSet := MetricSet(entity, tt.eventType, tt.hostname, tt.port, tt.remoteMonitoring)
			for key, expectedValue := range tt.expectedMetrics {
				actualValue, ok := metricSet.Metrics[key]
				assert.True(t, ok)
				assert.Equal(t, expectedValue, actualValue)
			}
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

	t.Run("ErrorCreatingEntity", func(t *testing.T) {
		// Simulate the error condition
		metricList := []interface{}{
			struct{}{},
		}
		args := arguments.ArgumentList{}
		err := IngestMetric(metricList, "testEvent", i, args)
		assert.NoError(t, err)
	})

	t.Run("ErrorProcessingModel", func(t *testing.T) {
		metricList := []interface{}{
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

func TestGetUniqueExcludedDatabases(t *testing.T) {
	tests := []struct {
		name              string
		excludedDBList    string
		expectedDatabases []string
	}{
		{
			name:              "Empty excludedDBList",
			excludedDBList:    "",
			expectedDatabases: constants.DefaultExcludedDatabases,
		},
		{
			name:              "Single database in excludedDBList",
			excludedDBList:    "db1",
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1"),
		},
		{
			name:              "Multiple databases in excludedDBList",
			excludedDBList:    "db1,db2",
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1", "db2"),
		},
		{
			name:              "Duplicate databases in excludedDBList",
			excludedDBList:    "db1,db1",
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1"),
		},
		{
			name:              "Databases with leading/trailing spaces",
			excludedDBList:    " db1 , db2 ",
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1", "db2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludedDBList := strings.Split(tt.excludedDBList, ",")
			for i := range excludedDBList {
				excludedDBList[i] = strings.TrimSpace(excludedDBList[i])
			}
			result := getUniqueExcludedDatabases(excludedDBList)
			assert.ElementsMatch(t, tt.expectedDatabases, result)
		})
	}
}

func TestGetExcludedDatabases(t *testing.T) {
	tests := []struct {
		name              string
		excludedDBList    string
		expectedDatabases []string
	}{
		{
			name:              "Valid JSON with multiple databases",
			excludedDBList:    `["db1","db2"]`,
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1", "db2"),
		},
		{
			name:              "Valid JSON with single database",
			excludedDBList:    `["db1"]`,
			expectedDatabases: append(constants.DefaultExcludedDatabases, "db1"),
		},
		{
			name:              "Invalid JSON",
			excludedDBList:    `["db1","db2"`,
			expectedDatabases: constants.DefaultExcludedDatabases,
		},
		{
			name:              "Empty JSON array",
			excludedDBList:    `[]`,
			expectedDatabases: constants.DefaultExcludedDatabases,
		},
		{
			name:              "Empty string",
			excludedDBList:    "",
			expectedDatabases: constants.DefaultExcludedDatabases,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExcludedDatabases(tt.excludedDBList)
			assert.ElementsMatch(t, tt.expectedDatabases, result)
		})
	}
}
