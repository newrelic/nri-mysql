package main

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	infrautils "github.com/newrelic/nri-mysql/src/infrautils"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
	"github.com/stretchr/testify/assert"
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

	var ms = metric.NewSet("eventType", nil)
	dbVersion := "5.6.0"
	populatePartialMetrics(ms, rawMetrics, metricDefinition, dbVersion)

	assert.Equal(t, 1., ms.Metrics["rawMetric1"])
	assert.Equal(t, 2., ms.Metrics["rawMetric2"])
	assert.Equal(t, "foo", ms.Metrics["rawMetric3"])
	assert.Nil(t, ms.Metrics["unknownMetric"])
	assert.Nil(t, ms.Metrics["badRawSource"])
	assert.Equal(t, 3., ms.Metrics["functionSource"])
}

func TestPopulateInventory(t *testing.T) {
	var rawInventory = map[string]interface{}{
		"key_1": 1,
		"key_2": 2,
		"key_3": "foo",
	}

	i := inventory.New()
	populateInventory(i, rawInventory)
	for key, value := range rawInventory {
		v, exists := i.Item(key)
		assert.True(t, exists)
		assert.Equal(t, value, v["value"])
	}
}

type testdb struct {
	inventory map[string]interface{}
	metrics   map[string]interface{}
	replica   map[string]interface{}
	version   map[string]interface{}
}

func (d testdb) close() {}
func (d testdb) query(query string) (map[string]interface{}, error) {
	if query == inventoryQuery {
		return d.inventory, nil
	}
	if query == metricsQuery {
		return d.metrics, nil
	}
	if query == replicaQueryBelowVersion8Point4 {
		return d.replica, nil
	}
	if query == dbVersionQuery {
		return d.version, nil
	}
	return nil, nil
}

func TestGetRawData(t *testing.T) {
	database := testdb{
		inventory: map[string]interface{}{
			"key_cache_block_size": 10,
			"key_buffer_size":      10,
			"version_comment":      "mysql",
			"version":              "5.6.3",
		},
		metrics: map[string]interface{}{},
		replica: map[string]interface{}{},
		version: map[string]interface{}{
			"version": "5.6.3",
		},
	}
	inventory, metrics, dbVersion, err := getRawData(database)
	if err != nil {
		t.Error()
	}
	if metrics == nil {
		t.Error()
	}
	if inventory == nil {
		t.Error()
	}
	if dbVersion == "" {
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
	ms := metric.NewSet("eventType", nil)
	dbVersion := "5.6.0"
	populatePartialMetrics(ms, rawMetrics, getDefaultMetrics(dbVersion), dbVersion)
	populatePartialMetrics(ms, rawMetrics, getExtendedMetrics(dbVersion), dbVersion)
	populatePartialMetrics(ms, rawMetrics, myisamMetrics, dbVersion)

	testMetrics := []string{"db.qCacheUtilization", "db.qCacheHitRatio", "db.threadCacheMissRate", "db.myisam.keyCacheUtilization"}

	expected := float64(0)
	for _, metricName := range testMetrics {
		actual := ms.Metrics[metricName]
		if actual != expected {
			assert.Equal(t, expected, actual, "For metric '%s', expected value: %f. Actual value: %f", metricName, expected, actual)
		}
	}
}

func TestPopulateMetricsOfTypePRATE(t *testing.T) {
	i, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	e, err := infrautils.CreateNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	infrautils.FatalIfErr(err)

	ms := infrautils.MetricSet(
		e,
		"MysqlSample",
		args.Hostname,
		args.Port,
		args.RemoteMonitoring,
	)

	rawMetrics := map[string]interface{}{
		"Created_tmp_files": 4500,
	}
	dbVersion := "5.6.0"
	populatePartialMetrics(ms, rawMetrics, getExtendedMetrics(dbVersion), dbVersion)
	//  db.createdTmpFilesPerSecond metric will be zero because there is no older value for this metric to calculate the PRATE.
	assert.Equal(t, float64(0), ms.Metrics["db.createdTmpFilesPerSecond"])
}
