package main

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/stretchr/testify/assert"
)

func TestBackupMetricsMap(t *testing.T) {
	// Test that all backup metrics are properly defined
	expectedMetrics := []string{
		"db.backupActive.total",
		"db.backupActive.logical",
		"db.backupActive.physical",
		"db.backupActive.tableLock",
	}

	for _, metricName := range expectedMetrics {
		t.Run(metricName, func(t *testing.T) {
			metricDef, exists := backupMetrics[metricName]
			assert.True(t, exists, "Metric %s should exist in backupMetrics map", metricName)
			assert.Equal(t, 2, len(metricDef), "Metric definition should have 2 elements")
			assert.IsType(t, "", metricDef[0], "First element should be a string (column name)")
			assert.Equal(t, metric.GAUGE, metricDef[1], "Second element should be metric.GAUGE")
		})
	}
}

func TestBackupMetricsQuery(t *testing.T) {
	// Test that the backup metrics query is properly defined
	assert.NotEmpty(t, backupMetricsQuery, "backupMetricsQuery should not be empty")

	// Test that query contains expected elements
	assert.Contains(t, backupMetricsQuery, "SELECT", "Query should contain SELECT")
	assert.Contains(t, backupMetricsQuery, "total_backup_active", "Query should count total backup active")
	assert.Contains(t, backupMetricsQuery, "logical_backup_active", "Query should count logical backups")
	assert.Contains(t, backupMetricsQuery, "physical_backup_active", "Query should count physical backups")
	assert.Contains(t, backupMetricsQuery, "table_lock_active", "Query should count table locks")
	assert.Contains(t, backupMetricsQuery, "performance_schema.metadata_locks", "Query should use metadata_locks for detection")
	assert.Contains(t, backupMetricsQuery, "information_schema.innodb_trx", "Query should detect logical backups")
	assert.Contains(t, backupMetricsQuery, "BACKUP_FTWRL%", "Query should detect physical backups via FTWRL backup lock")
	assert.Contains(t, backupMetricsQuery, "LOCK_TYPE", "Query should detect table locks")
	assert.Contains(t, backupMetricsQuery, "LOCK_DURATION IN ('EXPLICIT', 'TRANSACTION')", "Query should cover both MySQL (EXPLICIT) and MariaDB (TRANSACTION) LOCK TABLES behaviour")
	assert.Contains(t, backupMetricsQuery, "OWNER_THREAD_ID", "Query should identify backup sessions by thread ID")
}

func TestBackupMetricsColumnMapping(t *testing.T) {
	// Test that metric names correctly map to query column names
	tests := []struct {
		metricName string
		columnName string
	}{
		{"db.backupActive.total", "total_backup_active"},
		{"db.backupActive.logical", "logical_backup_active"},
		{"db.backupActive.physical", "physical_backup_active"},
		{"db.backupActive.tableLock", "table_lock_active"},
	}

	for _, test := range tests {
		t.Run(test.metricName, func(t *testing.T) {
			metricDef, exists := backupMetrics[test.metricName]
			assert.True(t, exists, "Metric %s should exist", test.metricName)
			assert.Equal(t, test.columnName, metricDef[0], "Column name should match for %s", test.metricName)
		})
	}
}

// Historical Backup Metrics Tests

func TestBackupHistoryMetricsMap(t *testing.T) {
	// Test that all backup history metrics are properly defined
	expectedMetrics := []string{
		"db.backupHistory.total",
		"db.backupHistory.logical.count",
		"db.backupHistory.logical.avgDuration",
		"db.backupHistory.logical.maxDuration",
		"db.backupHistory.physical.count",
		"db.backupHistory.physical.avgDuration",
		"db.backupHistory.physical.maxDuration",
		"db.backupHistory.tableLock.count",
		"db.backupHistory.tableLock.avgDuration",
		"db.backupHistory.tableLock.maxDuration",
	}

	for _, metricName := range expectedMetrics {
		t.Run(metricName, func(t *testing.T) {
			metricDef, exists := backupHistoryMetrics[metricName]
			assert.True(t, exists, "Metric %s should exist in backupHistoryMetrics map", metricName)
			assert.Equal(t, 2, len(metricDef), "Metric definition should have 2 elements")
			assert.IsType(t, "", metricDef[0], "First element should be a string (column name)")
			assert.Equal(t, metric.GAUGE, metricDef[1], "Second element should be metric.GAUGE")
		})
	}
}

func TestBackupHistoryMetricsQuery(t *testing.T) {
	// Test that the backup history metrics query is properly defined
	assert.NotEmpty(t, backupHistoryMetricsQuery, "backupHistoryMetricsQuery should not be empty")

	// Test that query contains expected elements
	assert.Contains(t, backupHistoryMetricsQuery, "SELECT", "Query should contain SELECT")
	assert.Contains(t, backupHistoryMetricsQuery, "total_backup_history", "Query should count total backup history")
	assert.Contains(t, backupHistoryMetricsQuery, "logical_backup_count", "Query should count logical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "logical_backup_avg_duration", "Query should calculate avg duration for logical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "logical_backup_max_duration", "Query should calculate max duration for logical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "physical_backup_count", "Query should count physical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "physical_backup_avg_duration", "Query should calculate avg duration for physical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "physical_backup_max_duration", "Query should calculate max duration for physical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "table_lock_count", "Query should count table locks")
	assert.Contains(t, backupHistoryMetricsQuery, "table_lock_avg_duration", "Query should calculate avg duration for table locks")
	assert.Contains(t, backupHistoryMetricsQuery, "table_lock_max_duration", "Query should calculate max duration for table locks")
	assert.Contains(t, backupHistoryMetricsQuery, "performance_schema.events_statements_history", "Query should use events_statements_history")
	assert.Contains(t, backupHistoryMetricsQuery, "START TRANSACTION WITH CONSISTENT SNAPSHOT", "Query should detect logical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "FLUSH TABLES%WITH READ LOCK", "Query should detect physical backups")
	assert.Contains(t, backupHistoryMetricsQuery, "LOCK TABLES %", "Query should detect table locks without matching UNLOCK TABLES")
	assert.Contains(t, backupHistoryMetricsQuery, "TIMER_WAIT/1000000000000", "Query should convert timer to seconds")
}

func TestBackupHistoryMetricsColumnMapping(t *testing.T) {
	// Test that metric names correctly map to query column names
	tests := []struct {
		metricName string
		columnName string
	}{
		{"db.backupHistory.total", "total_backup_history"},
		{"db.backupHistory.logical.count", "logical_backup_count"},
		{"db.backupHistory.logical.avgDuration", "logical_backup_avg_duration"},
		{"db.backupHistory.logical.maxDuration", "logical_backup_max_duration"},
		{"db.backupHistory.physical.count", "physical_backup_count"},
		{"db.backupHistory.physical.avgDuration", "physical_backup_avg_duration"},
		{"db.backupHistory.physical.maxDuration", "physical_backup_max_duration"},
		{"db.backupHistory.tableLock.count", "table_lock_count"},
		{"db.backupHistory.tableLock.avgDuration", "table_lock_avg_duration"},
		{"db.backupHistory.tableLock.maxDuration", "table_lock_max_duration"},
	}

	for _, test := range tests {
		t.Run(test.metricName, func(t *testing.T) {
			metricDef, exists := backupHistoryMetrics[test.metricName]
			assert.True(t, exists, "Metric %s should exist", test.metricName)
			assert.Equal(t, test.columnName, metricDef[0], "Column name should match for %s", test.metricName)
		})
	}
}

func TestBackupMetricsCount(t *testing.T) {
	// Verify the expected number of metrics
	assert.Equal(t, 4, len(backupMetrics), "Should have 4 active backup metrics")
	assert.Equal(t, 10, len(backupHistoryMetrics), "Should have 10 historical backup metrics")
}
