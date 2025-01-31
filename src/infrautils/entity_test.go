package infrautils

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/stretchr/testify/assert"
)

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
