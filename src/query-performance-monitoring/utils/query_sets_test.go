package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetQuerySet(t *testing.T) {
	tests := []struct {
		name                    string
		flavor                  DatabaseFlavor
		expectedQuery           string
		needsQueryAnonymization bool
		shouldContain           []string
		shouldNotContain        []string
	}{
		{
			name:                    "MySQL flavor returns MySQL query with CPU time",
			flavor:                  DatabaseFlavorMySQL,
			expectedQuery:           SlowQueries,
			needsQueryAnonymization: false,
			shouldContain: []string{
				"SUM_CPU_TIME / COUNT_STAR",
				"CONVERT_TZ(LAST_SEEN, @@session.time_zone, '+00:00')",
			},
			shouldNotContain: []string{
				"NULL AS avg_cpu_time_ms",
				"NULLIF(COUNT_STAR, 0)",
			},
		},
		{
			name:                    "MariaDB flavor returns MariaDB query without CPU time",
			flavor:                  DatabaseFlavorMariaDB,
			expectedQuery:           MariaDBSlowQueries,
			needsQueryAnonymization: true,
			shouldContain: []string{
				"NULL AS avg_cpu_time_ms",
				"CONVERT_TZ(LAST_SEEN, @@session.time_zone, '+00:00')",
			},
			shouldNotContain: []string{
				"SUM_CPU_TIME / COUNT_STAR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			querySet := GetQuerySet(tt.flavor)

			// Verify the correct query is selected
			assert.Equal(t, tt.expectedQuery, querySet.SlowQueries)

			// Verify anonymization flag is set correctly per flavor
			assert.Equal(t, tt.needsQueryAnonymization, querySet.NeedsQueryAnonymization,
				"NeedsQueryAnonymization should be %v for %s", tt.needsQueryAnonymization, tt.name)

			// Verify the query contains expected elements
			for _, expectedContent := range tt.shouldContain {
				assert.Contains(t, querySet.SlowQueries, expectedContent,
					"Query should contain %s", expectedContent)
			}

			// Verify the query does not contain forbidden elements
			for _, forbiddenContent := range tt.shouldNotContain {
				assert.NotContains(t, querySet.SlowQueries, forbiddenContent,
					"Query should not contain %s", forbiddenContent)
			}

			// Verify other queries are consistent across flavors
			assert.Equal(t, CurrentRunningQueriesSearch, querySet.CurrentRunningQueriesSearch)
			assert.Equal(t, RecentQueriesSearch, querySet.RecentQueriesSearch)
		})
	}
}

func TestMariaDBQueryStructure(t *testing.T) {
	querySet := GetQuerySet(DatabaseFlavorMariaDB)
	mariaDBQuery := querySet.SlowQueries

	// Test that CPU time is explicitly NULL in MariaDB
	assert.Contains(t, mariaDBQuery, "NULL AS avg_cpu_time_ms",
		"MariaDB query should explicitly return NULL for CPU time")

	// Test that MariaDB query uses CONVERT_TZ for timezone handling (same as MySQL)
	assert.Contains(t, mariaDBQuery, "CONVERT_TZ(LAST_SEEN, @@session.time_zone, '+00:00')",
		"MariaDB query should use CONVERT_TZ for consistent timezone handling")

	// Test that the query structure is consistent with MySQL query
	mysqlQuery := GetQuerySet(DatabaseFlavorMySQL).SlowQueries

	// Both queries should have similar structure
	assert.Contains(t, mariaDBQuery, "SELECT", "MariaDB query should be a SELECT statement")
	assert.Contains(t, mariaDBQuery, "FROM performance_schema.events_statements_summary_by_digest",
		"MariaDB query should use performance_schema")
	assert.Contains(t, mariaDBQuery, "ORDER BY avg_elapsed_time_ms DESC",
		"MariaDB query should order by elapsed time")
	assert.Contains(t, mariaDBQuery, "LIMIT ?", "MariaDB query should support LIMIT")

	// Count the number of selected columns (should be same as MySQL)
	mysqlSelectColumns := strings.Count(mysqlQuery, " AS ")
	mariaDBSelectColumns := strings.Count(mariaDBQuery, " AS ")
	assert.Equal(t, mysqlSelectColumns, mariaDBSelectColumns,
		"MariaDB and MySQL queries should select the same number of columns")
}

func TestQueryParameterConsistency(t *testing.T) {
	mysqlQuerySet := GetQuerySet(DatabaseFlavorMySQL)
	mariaDBQuerySet := GetQuerySet(DatabaseFlavorMariaDB)

	// Both queries should have the same number of parameters (?)
	mysqlParams := strings.Count(mysqlQuerySet.SlowQueries, "?")
	mariaDBParams := strings.Count(mariaDBQuerySet.SlowQueries, "?")

	assert.Equal(t, mysqlParams, mariaDBParams,
		"MySQL and MariaDB slow queries should have the same number of parameters")

	// Both should have 3 parameters: interval, excluded databases, limit
	assert.Equal(t, 3, mysqlParams, "Slow query should have 3 parameters")
	assert.Equal(t, 3, mariaDBParams, "MariaDB slow query should have 3 parameters")
}

func TestMariaDBBlockingSessionsQueryStructure(t *testing.T) {
	mariaDBQuery := GetQuerySet(DatabaseFlavorMariaDB).BlockingSessionsQuery
	mysqlQuery := GetQuerySet(DatabaseFlavorMySQL).BlockingSessionsQuery

	// MariaDB uses CTE to pre-join threads + events_statements_current once for both sides
	assert.Contains(t, mariaDBQuery, "WITH thread_stmt AS",
		"MariaDB blocking query should use CTE thread_stmt")
	assert.Contains(t, mariaDBQuery, "performance_schema.events_statements_current",
		"MariaDB blocking query CTE should join events_statements_current")

	// MariaDB uses COALESCE to fall back to raw trx_query when DIGEST_TEXT is unavailable
	assert.Contains(t, mariaDBQuery, "COALESCE(wt.DIGEST_TEXT, r.trx_query)",
		"MariaDB blocking query should use COALESCE with trx_query fallback for blocked_query")
	assert.Contains(t, mariaDBQuery, "COALESCE(bt.DIGEST_TEXT, b.trx_query)",
		"MariaDB blocking query should use COALESCE with trx_query fallback for blocking_query")

	// blocking_status must never be NULL — idle-in-transaction blocking threads have NULL PROCESSLIST_STATE
	assert.Contains(t, mariaDBQuery, "COALESCE(bt.PROCESSLIST_STATE, 'Idle in transaction')",
		"MariaDB blocking query should default blocking_status to 'Idle in transaction' when NULL")

	// REGEXP_REPLACE must NOT appear — anonymization is handled in Go, not SQL
	assert.NotContains(t, mariaDBQuery, "REGEXP_REPLACE",
		"MariaDB blocking query should not contain REGEXP_REPLACE; anonymization is done in Go")

	// MariaDB uses information_schema.innodb_lock_waits (not performance_schema.data_lock_waits)
	assert.Contains(t, mariaDBQuery, "information_schema.innodb_lock_waits",
		"MariaDB blocking query should use information_schema.innodb_lock_waits")
	assert.NotContains(t, mariaDBQuery, "performance_schema.data_lock_waits",
		"MariaDB blocking query should not use performance_schema.data_lock_waits")

	// MariaDB query should have exactly 2 bind parameters (excluded DBs + limit)
	mariaDBParams := strings.Count(mariaDBQuery, "?")
	assert.Equal(t, 2, mariaDBParams,
		"MariaDB blocking query should have exactly 2 bind parameters")

	// MySQL uses performance_schema.data_lock_waits
	assert.Contains(t, mysqlQuery, "performance_schema.data_lock_waits",
		"MySQL blocking query should use performance_schema.data_lock_waits")

	// MySQL does not need the trx_query COALESCE fallback
	assert.NotContains(t, mysqlQuery, "COALESCE(es_waiting.DIGEST_TEXT, r.trx_query)",
		"MySQL blocking query should not need trx_query fallback")
}

func TestMariaDBQueryFieldMapping(t *testing.T) {
	querySet := GetQuerySet(DatabaseFlavorMariaDB)
	query := querySet.SlowQueries

	// Test that all expected fields are present in MariaDB query
	expectedFields := []string{
		"query_id",
		"query_text",
		"database_name",
		"schema_name",
		"execution_count",
		"avg_cpu_time_ms",
		"avg_elapsed_time_ms",
		"avg_disk_reads",
		"avg_disk_writes",
		"has_full_table_scan",
		"statement_type",
		"last_execution_timestamp",
		"collection_timestamp",
	}

	for _, field := range expectedFields {
		assert.Contains(t, query, "AS "+field,
			"MariaDB query should include field %s", field)
	}
}
