package utils

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"

	"github.com/stretchr/testify/assert"
)

var args arguments.ArgumentList

const (
	integrationVersion = "0.0.0"
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
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON input: %v", err)
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			result := GetExcludedDatabases(tt.ExcludedDBList)
			assert.ElementsMatch(t, tt.ExpectedDatabases, result)
		})
	}
}

func TestGenerateDSNPriorizesCliOverEnvArgs(t *testing.T) {
	os.Setenv("USERNAME", "dbuser")
	os.Setenv("HOSTNAME", "foo")

	os.Args = []string{
		"cmd",
		"-hostname=bar",
		"-port=1234",
		"-password=dbpwd",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(bar:1234)/?", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsOldPasswords(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-old_passwords",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?allowOldPasswords=true", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsEnableTLS(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-enable_tls",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?tls=true", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsInsecureSkipVerify(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-insecure_skip_verify",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?tls=skip-verify", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsExtraConnectionURLArgs(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-extra_connection_url_args=readTimeout=1s&timeout=5s&tls=skip-verify",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?readTimeout=1s&timeout=5s&tls=skip-verify", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSocketDiscardPort(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-socket=/path/to/socket/file",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@unix(/path/to/socket/file)/?", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}
