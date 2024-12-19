package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/blang/semver/v4"
	"github.com/newrelic/infra-integrations-sdk/v3/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

const (
	inventoryQuery  = "SHOW GLOBAL VARIABLES"
	metricsQuery    = "SHOW /*!50002 GLOBAL */ STATUS"
	replicaQuery560 = "SHOW SLAVE STATUS"
	replicaQuery800 = "SHOW REPLICA STATUS"
	versionQuery    = "SELECT VERSION() as version;"
)

var errVersionNotFound = errors.New("version not found in versionQueryResult")

func getReplicaQuery(version *semver.Version) string {
	if version.GE(semver.Version{Major: 8, Minor: 0, Patch: 0}) {
		return replicaQuery800
	} else {
		return replicaQuery560
	}
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

func getRawData(db dataSource) (map[string]interface{}, map[string]interface{}, *semver.Version, error) {
	version, err := collectVersion(db)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("metrics collection failed: error collecting version number: %w", err)
	}

	inventory, err := db.query(inventoryQuery)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error querying inventory: %w", err)
	}
	metrics, err := db.query(metricsQuery)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error querying metrics: %w", err)
	}

	replicaQuery := getReplicaQuery(version)
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

	return inventory, metrics, version, nil
}

func populateInventory(inventory *inventory.Inventory, rawData map[string]interface{}) {
	for name, value := range rawData {
		err := inventory.SetItem(name, "value", value)
		if err != nil {
			log.Warn("cannot add item %s to inventory: %v", name, err)
		}
	}
}

func populateMetrics(sample *metric.Set, rawMetrics map[string]interface{}, version *semver.Version) {
	defaultMetrics := getDefaultMetrics(version)
	if rawMetrics["node_type"] != "slave" {
		delete(defaultMetrics, "cluster.slaveRunning")
	}
	populatePartialMetrics(sample, rawMetrics, defaultMetrics, version)

	if args.ExtendedMetrics {
		extendedMetrics := getExtendedMetrics(version)
		if rawMetrics["node_type"] == "slave" {
			slaveMetrics := getSlaveMetrics(version)
			for key := range slaveMetrics {
				extendedMetrics[key] = slaveMetrics[key]
			}
		}
		populatePartialMetrics(sample, rawMetrics, extendedMetrics, version)
	}
	if args.ExtendedInnodbMetrics {
		populatePartialMetrics(sample, rawMetrics, innodbMetrics, version)
	}
	if args.ExtendedMyIsamMetrics {
		populatePartialMetrics(sample, rawMetrics, myisamMetrics, version)
	}

}
func populatePartialMetrics(ms *metric.Set, metrics map[string]interface{}, metricsDefinition map[string][]interface{}, version *semver.Version) {
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
		case func(map[string]interface{}, *semver.Version) (int, bool):
			rawMetric, ok = source(metrics, version)
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

func collectVersion(db dataSource) (*semver.Version, error) {
	versionQueryResult, err := db.query(versionQuery)
	if err != nil {
		return nil, fmt.Errorf("error fetching version: %w", err)
	}

	if versionStr, exists := versionQueryResult["version"]; exists {
		version := versionStr.(string)
		log.Debug("Original MySQL Server version string:", version)

		semVersion, err := semver.ParseTolerant(version)
		if err != nil {
			return nil, err
		}

		log.Debug("Parsed version as semver:", semVersion)
		return &semVersion, nil
	} else {
		return nil, fmt.Errorf("%w", errVersionNotFound)
	}
}
