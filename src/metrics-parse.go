package main

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"io/ioutil"
	"strconv"
	"sync"

	"github.com/newrelic/infra-integrations-sdk/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/log"
	"gopkg.in/yaml.v2"
)

const (
	inventoryQuery = "SHOW GLOBAL VARIABLES"
	metricsQuery   = "SHOW /*!50002 GLOBAL */ STATUS"
	replicaQuery   = "SHOW SLAVE STATUS"
)

type customQuery struct {
	Query    string
	Prefix   string
	Name     string `yaml:"metric_name"`
	Type     string `yaml:"metric_type"`
	Database string
}

// Try to convert a string to its type or return the string if not possible
func asValue(value string) interface{} {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}

	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	return value
}

func getRawData(db dataSource) (map[string]interface{}, map[string]interface{}, error) {
	inventory, err := db.query(inventoryQuery)
	if err != nil {
		return nil, nil, err
	}
	metrics, err := db.query(metricsQuery)
	if err != nil {
		return nil, nil, err
	}

	replication, err := db.query(replicaQuery)
	if err != nil {
		log.Warn("Can't get node type, not enough privileges (must grant REPLICATION CLIENT)")
	} else if len(replication) == 0 {
		metrics["node_type"] = "master"
	} else {
		metrics["node_type"] = "slave"

		for key := range replication {
			metrics[key] = replication[key]
		}
	}

	metrics["key_cache_block_size"] = inventory["key_cache_block_size"]
	metrics["key_buffer_size"] = inventory["key_buffer_size"]
	metrics["version_comment"] = inventory["version_comment"]
	metrics["version"] = inventory["version"]

	return inventory, metrics, nil
}

func getPopulateCustomData(db dataSource, e *integration.Entity) {
	var err error

	if len(args.CustomMetricsQuery) > 0 {
		if err = populateCustomMetrics(db, e, customQuery{Query: args.CustomMetricsQuery}); err != nil {
			log.Error("CustomMetrics %s", err)
		}
	} else if len(args.CustomMetricsConfig) > 0 {
		queries, err := parseCustomQueries()
		if err != nil {
			log.Error("Failed to parse custom queries: %s", err)
		}
		var wg sync.WaitGroup
		for _, query := range queries {
			wg.Add(1)
			go func(query customQuery) {
				defer wg.Done()
				if err = populateCustomMetrics(db, e, query); err != nil {
					log.Error("CustomMetrics %s", err)
				}
			}(query)
		}
		wg.Wait()
	}
}

func parseCustomQueries() ([]customQuery, error) {
	// load YAML config file
	b, err := ioutil.ReadFile(args.CustomMetricsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read custom_metrics_config: %s", err)
	}
	// parse
	var c struct{ Queries []customQuery }
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return nil, fmt.Errorf("failed to parse custom_metrics_config: %s", err)
	}

	return c.Queries, nil
}

func populateCustomMetrics(db dataSource, e *integration.Entity, query customQuery) error {
	// Multi-row query
	rawRows, err := db.queryRows(query.Query)
	if err != nil {
		return fmt.Errorf("db.queryRows: %s", err)
	}

	for _, row := range rawRows {
		nameInterface, ok := row["metric_name"]
		var name string
		if !ok {
			if len(query.Name) > 0 {
				name = query.Name
			}
		} else {
			name, ok = nameInterface.(string)
			if !ok {
				return fmt.Errorf("Non-string type %T for custom query 'metric_name' column", nameInterface)
			}
		}

		value, ok := row["metric_value"]
		var valueString string
		if !ok {
			if len(name) > 0 {
				return fmt.Errorf("Missing 'metric_value' for %s in custom query", name)
			}
		} else {
			valueString = fmt.Sprintf("%v", value)
			if len(name) == 0 {
				return fmt.Errorf("Missing 'metric_name' for %s in custom query", valueString)
			}
		}

		if len(query.Prefix) > 0 {
			name = query.Prefix + name
		}

		var metricType metric.SourceType
		metricTypeInterface, ok := row["metric_type"]
		if !ok {
			if len(query.Type) > 0 {
				metricType, err = metric.SourceTypeForName(query.Type)
				if err != nil {
					return fmt.Errorf("Invalid metric type %s in YAML: %s", query.Type, err)
				}
			} else {
				metricType = detectMetricType(valueString)
			}
		} else {
			// metric type was specified
			metricTypeString, ok := metricTypeInterface.(string)
			if !ok {
				return fmt.Errorf("Non-string type %T for custom query 'metric_type' column", metricTypeInterface)
			}
			metricType, err = metric.SourceTypeForName(metricTypeString)
			if err != nil {
				return fmt.Errorf("Invalid metric type %s in query 'metric_type' column: %s", metricTypeString, err)
			}
		}

		ms := metricSet(
			e,
			"MysqlCustomQuerySample",
			args.Hostname,
			args.Port,
			args.RemoteMonitoring,
		)
		attributes := []metric.Attribute{}
		if len(query.Database) > 0 {
			attributes = append(attributes, metric.Attribute{Key: "database", Value: query.Database})
		}

		for k, v := range row {
			if k == "metric_name" || k == "metric_type" || k == "metric_value" {
				continue
			}
			vString := fmt.Sprintf("%v", v)

			if len(query.Prefix) > 0 {
				k = query.Prefix + k
			}

			err = ms.SetMetric(k, vString, detectMetricType(vString))
			if err != nil {
				log.Error("Failed to set metric: %s", err)
				continue
			}
		}

		if len(valueString) > 0 {
			err = ms.SetMetric(name, valueString, metricType)
			if err != nil {
				log.Error("Failed to set metric: %s", err)
			}
		}
	}
	return nil
}

func detectMetricType(value string) metric.SourceType {
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return metric.ATTRIBUTE
	}

	return metric.GAUGE
}

func populateInventory(inventory *inventory.Inventory, rawData map[string]interface{}) {
	for name, value := range rawData {
		inventory.SetItem(name, "value", value)
	}
}

func populateMetrics(sample *metric.Set, rawMetrics map[string]interface{}) {
	if rawMetrics["node_type"] != "slave" {
		delete(defaultMetrics, "cluster.slaveRunning")
	}
	populatePartialMetrics(sample, rawMetrics, defaultMetrics)

	if args.ExtendedMetrics {
		if rawMetrics["node_type"] == "slave" {
			for key := range slaveMetrics {
				extendedMetrics[key] = slaveMetrics[key]
			}
		}
		populatePartialMetrics(sample, rawMetrics, extendedMetrics)
	}
	if args.ExtendedInnodbMetrics {
		populatePartialMetrics(sample, rawMetrics, innodbMetrics)
	}
	if args.ExtendedMyIsamMetrics {
		populatePartialMetrics(sample, rawMetrics, myisamMetrics)
	}

}

func populatePartialMetrics(ms *metric.Set, metrics map[string]interface{}, metricsDefinition map[string][]interface{}) {
	for metricName, metricConf := range metricsDefinition {
		rawSource := metricConf[0]
		metricType := metricConf[1].(metric.SourceType)

		var rawMetric interface{}
		var ok bool

		switch source := rawSource.(type) {
		case string:
			rawMetric, ok = metrics[source]
		case func(map[string]interface{}) (float64, bool):
			rawMetric, ok = source(metrics)
		case func(map[string]interface{}) (int, bool):
			rawMetric, ok = source(metrics)
		default:
			log.Warn("Invalid raw source metric for %s", metricName)
			continue
		}

		if !ok {
			log.Warn("Can't find raw metrics in results for %s", metricName)
			continue
		}

		err := ms.SetMetric(metricName, rawMetric, metricType)

		if err != nil {
			log.Warn("Error setting value: %s", err)
			continue
		}
	}
}
