package main

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/newrelic/infra-integrations-sdk/sdk"
)

func TestAsValue(t *testing.T) {
	intValue, ok := asValue("10").(int)
	if ok != true {
		t.Error()
	}
	if intValue != 10 {
		t.Error()
	}

	floatValue, ok := asValue("0.12").(float64)
	if ok != true {
		t.Error()
	}
	if floatValue != 0.12 {
		t.Error()
	}

	boolValue, ok := asValue("true").(bool)
	if ok != true {
		t.Error()
	}
	if boolValue != true {
		t.Error()
	}

	stringValue, ok := asValue("test string").(string)
	if ok != true {
		t.Error()
	}
	if stringValue != "test string" {
		t.Error()
	}
}

func TestPopulatePartialMetrics(t *testing.T) {
	var rawMetrics = map[string]interface{}{
		"raw_metric_1": 1,
		"raw_metric_2": 2,
		"raw_metric_3": "foo",
	}

	functionSource := func(a map[string]interface{}) (float64, bool) {
		return float64(a["raw_metric_1"].(int) + a["raw_metric_2"].(int)), true
	}

	var metricDefinition = map[string][]interface{}{
		"rawMetric1":     {"raw_metric_1", metric.GAUGE},
		"rawMetric2":     {"raw_metric_2", metric.GAUGE},
		"rawMetric3":     {"raw_metric_3", metric.ATTRIBUTE},
		"unknownMetric":  {"raw_metric_4", metric.GAUGE},
		"badRawSource":   {10, metric.GAUGE},
		"functionSource": {functionSource, metric.GAUGE},
	}

	var sample = metric.NewMetricSet("eventType")
	populatePartialMetrics(&sample, rawMetrics, metricDefinition)

	if sample["rawMetric1"] != 1 {
		t.Error()
	}
	if sample["rawMetric2"] != 2 {
		t.Error()
	}
	if sample["rawMetric3"] != "foo" {
		t.Error()
	}

	if sample["unknownMetric"] != nil {
		t.Error()
	}
	if sample["badRawSource"] != nil {
		t.Error()
	}
	if sample["functionSource"] != float64(3) {
		t.Error()
	}

}

func TestPopulateInventory(t *testing.T) {
	var rawInventory = map[string]interface{}{
		"key_1": 1,
		"key_2": 2,
		"key_3": "foo",
	}

	inventory := make(sdk.Inventory)
	populateInventory(inventory, rawInventory)
	for key, value := range rawInventory {
		if inventory[key]["value"] != value {
			t.Error()
		}
	}
}

type testdb struct {
	inventory map[string]interface{}
	metrics   map[string]interface{}
	replica   map[string]interface{}
}

func (d testdb) close() {}
func (d testdb) query(query string) (map[string]interface{}, error) {
	if query == inventoryQuery {
		return d.inventory, nil
	}
	if query == metricsQuery {
		return d.metrics, nil
	}
	if query == replicaQuery {
		return d.replica, nil
	}
	return nil, nil
}

func TestGetRawData(t *testing.T) {
	database := testdb{
		inventory: map[string]interface{}{
			"key_cache_block_size": 10,
			"key_buffer_size":      10,
			"version_comment":      "mysql",
			"version":              "5.4.3",
		},
		metrics: map[string]interface{}{},
		replica: map[string]interface{}{},
	}
	inventory, metrics, err := getRawData(database)
	if err != nil {
		t.Error()
	}
	if metrics == nil {
		t.Error()
	}
	if inventory == nil {
		t.Error()
	}
}

func TestPopulateMetricsWithZeroValuesInData(t *testing.T) {
	rawMetrics := map[string]interface{}{
		"Qcache_free_blocks":   0,
		"Qcache_total_blocks":  0,
		"Qcache_not_cached":    0,
		"Qcache_hits":          0,
		"Queries":              0,
		"Threads_created":      0,
		"Connections":          0,
		"Key_blocks_unused":    0,
		"Key_cache_block_size": 0,
		"Key_buffer_size":      0,
	}
	ms := metric.NewMetricSet("eventType")
	populatePartialMetrics(&ms, rawMetrics, defaultMetrics)
	populatePartialMetrics(&ms, rawMetrics, extendedMetrics)
	populatePartialMetrics(&ms, rawMetrics, myisamMetrics)

	testMetrics := []string{"db.qCacheUtilization", "db.qCacheHitRatio", "db.threadCacheMissRate", "db.myisam.keyCacheUtilization"}

	expected := float64(0)
	for _, metricName := range testMetrics {
		actual, _ := ms[metricName]
		if actual != expected {
			t.Errorf("For metric '%s', expected value: %f. Actual value: %f", metricName, expected, actual)
		}
	}
}
