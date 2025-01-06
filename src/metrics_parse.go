package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

const (
	inventoryQuery                  = "SHOW GLOBAL VARIABLES"
	metricsQuery                    = "SHOW /*!50002 GLOBAL */ STATUS"
	replicaQueryBelowVersion8Point4 = "SHOW SLAVE STATUS"
	/*
		From Mysql 8.4 SHOW SLAVE STATUS is removed and SHOW REPLICA STATUS should be used instead
		Ref - https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-0.html#:~:text=SQL%20statements%20removed
	*/
	replicaQueryForVersion8Point4AndAbove = "SHOW REPLICA STATUS"
	dbVersionQuery                        = "SELECT VERSION() as version;"

	dbMajorVersionThreshold = 8
	dbMinorVersionThreshold = 4
)

var errVersionNotFound = errors.New("version not found in versionQueryResult")
var errSemanticVersionNotFound = errors.New("semantic version not found")

func isDBVersionLessThan8(dbVersion string) bool {
	parts := strings.Split(dbVersion, ".")

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Warn("Could not convert major version from str to int. Assuming to be less than version 8.4 and returning")
		return true
	}

	return majorVersion < dbMajorVersionThreshold
}

func isDBVersionLessThan8Point4(dbVersion string) bool {
	parts := strings.Split(dbVersion, ".")

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Warn("Could not convert major version from str to int. Assuming to be less than version 8.4 and returning")
		return true
	}

	minorVersion, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Warn("Could not convert minor version from str to int. Assuming to be less than version 8.4 and returning")
		return true
	}
	return majorVersion < dbMajorVersionThreshold || (majorVersion == dbMajorVersionThreshold && minorVersion < dbMinorVersionThreshold)
}

func getReplicaQuery(dbVersion string) string {
	if isDBVersionLessThan8Point4(dbVersion) {
		return replicaQueryBelowVersion8Point4
	}
	return replicaQueryForVersion8Point4AndAbove
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

func getRawData(db dataSource) (map[string]interface{}, map[string]interface{}, string, error) {
	dbVersion, err := collectDBVersion(db)
	if err != nil {
		log.Warn(err.Error())
		log.Warn("Assuming the mysql version to be less than 8.4 and proceeding further")
		dbVersion = "5.7.0"
	}

	inventory, err := db.query(inventoryQuery)
	if err != nil {
		return nil, nil, "", fmt.Errorf("error querying inventory: %w", err)
	}
	metrics, err := db.query(metricsQuery)
	if err != nil {
		return nil, nil, "", fmt.Errorf("error querying metrics: %w", err)
	}

	replicaQuery := getReplicaQuery(dbVersion)
	switch replication, err := db.query(replicaQuery); {
	case err != nil:
		log.Warn("Can't get node type, not enough privileges (must grant REPLICATION CLIENT)")
	case len(replication) == 0:
		metrics["node_type"] = "master"
	default:
		metrics["node_type"] = "slave"
		for key := range replication {
			metrics[key] = replication[key]
		}
	}

	metrics["key_cache_block_size"] = inventory["key_cache_block_size"]
	metrics["key_buffer_size"] = inventory["key_buffer_size"]
	metrics["version_comment"] = inventory["version_comment"]
	metrics["version"] = inventory["version"]

	return inventory, metrics, dbVersion, nil
}

func populateInventory(inventory *inventory.Inventory, rawData map[string]interface{}) {
	for name, value := range rawData {
		err := inventory.SetItem(name, "value", value)
		if err != nil {
			log.Warn("cannot add item %s to inventory: %v", name, err)
		}
	}
}

func populateMetrics(sample *metric.Set, rawMetrics map[string]interface{}, dbVersion string) {
	defaultMetrics := getDefaultMetrics(dbVersion)
	if rawMetrics["node_type"] != "slave" {
		delete(defaultMetrics, "cluster.slaveRunning")
	}
	populatePartialMetrics(sample, rawMetrics, defaultMetrics, dbVersion)

	if args.ExtendedMetrics {
		extendedMetrics := getExtendedMetrics(dbVersion)
		if rawMetrics["node_type"] == "slave" {
			slaveMetrics := getSlaveMetrics(dbVersion)
			for key := range slaveMetrics {
				extendedMetrics[key] = slaveMetrics[key]
			}
		}
		populatePartialMetrics(sample, rawMetrics, extendedMetrics, dbVersion)
	}
	if args.ExtendedInnodbMetrics {
		populatePartialMetrics(sample, rawMetrics, innodbMetrics, dbVersion)
	}
	if args.ExtendedMyIsamMetrics {
		populatePartialMetrics(sample, rawMetrics, myisamMetrics, dbVersion)
	}
}

func populatePartialMetrics(ms *metric.Set, metrics map[string]interface{}, metricsDefinition map[string][]interface{}, dbVersion string) {
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
		case func(map[string]interface{}, string) (int, bool):
			rawMetric, ok = source(metrics, dbVersion)
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

func collectDBVersion(db dataSource) (string, error) {
	versionQueryResult, err := db.query(dbVersionQuery)
	if err != nil {
		return "", fmt.Errorf("error fetching dbVersion: %w", err)
	}

	if versionStr, exists := versionQueryResult["version"]; exists {
		version := versionStr.(string)
		log.Debug("Original MySQL Server version string: %s", version)

		sanitizedVersion, err := extractSanitizedVersion(version)
		if err != nil {
			return "", fmt.Errorf("error extracting version: %w", err)
		}

		log.Debug("sanitized version: %v", sanitizedVersion)
		return sanitizedVersion, nil
	} else {
		return "", fmt.Errorf("%w", errVersionNotFound)
	}
}

// extractSanitizedVersion uses a regular expression to extract a version string up to major.minor.patch
func extractSanitizedVersion(version string) (string, error) {
	reg := regexp.MustCompile(`^(?P<major>\d+)(?:\.(?P<minor>\d+))?(?:\.(?P<patch>\d+))?`)
	matches := reg.FindStringSubmatch(version)

	if len(matches) == 0 || matches[1] == "" {
		return "", errSemanticVersionNotFound
	}

	major := matches[1]
	minor := "0"
	patch := "0"

	if len(matches) > 2 && matches[2] != "" {
		minor = matches[2]
	}
	if len(matches) > 3 && matches[3] != "" {
		patch = matches[3]
	}

	return fmt.Sprintf("%s.%s.%s", major, minor, patch), nil
}
