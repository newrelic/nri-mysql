package utils

const (
	/*
		SlowQueries: Retrieves a list of slow queries that have been executed within a certain period.
		This query provides insights into slow-performing queries by capturing metrics such as average CPU time,
		average elapsed time, and disk reads/writes. It's beneficial for identifying resource-intensive queries
		that may require optimization, such as indexing or query restructuring.

		Arguments:
		1. Interval in seconds (INT): The time period to look back for slow queries.
		2. Excluded databases (STRING): A comma-separated list of database names to exclude from the results.
		3. Limit (INT): The maximum number of results to return.
	*/
	SlowQueries = `
        SELECT
			DIGEST AS query_id,
			CASE
				WHEN CHAR_LENGTH(DIGEST_TEXT) > 4000 THEN CONCAT(LEFT(DIGEST_TEXT, 3997), '...')
				ELSE DIGEST_TEXT
			END AS query_text,
			SCHEMA_NAME AS database_name,
			'N/A' AS schema_name,
			COUNT_STAR AS execution_count,
			ROUND((SUM_CPU_TIME / COUNT_STAR) / 1000000000, 3) AS avg_cpu_time_ms,
			ROUND((SUM_TIMER_WAIT / COUNT_STAR) / 1000000000, 3) AS avg_elapsed_time_ms,
			SUM_ROWS_EXAMINED / COUNT_STAR AS avg_disk_reads,
			SUM_ROWS_AFFECTED / COUNT_STAR AS avg_disk_writes,
			CASE
				WHEN SUM_NO_INDEX_USED > 0 THEN 'Yes'
				ELSE 'No'
			END AS has_full_table_scan,
			CASE
				WHEN DIGEST_TEXT LIKE 'SELECT%' THEN 'SELECT'
				WHEN DIGEST_TEXT LIKE 'INSERT%' THEN 'INSERT'
				WHEN DIGEST_TEXT LIKE 'UPDATE%' THEN 'UPDATE'
				WHEN DIGEST_TEXT LIKE 'DELETE%' THEN 'DELETE'
				ELSE 'OTHER'
			END AS statement_type,
			DATE_FORMAT(LAST_SEEN, '%Y-%m-%dT%H:%i:%sZ') AS last_execution_timestamp,
			DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
		FROM performance_schema.events_statements_summary_by_digest
		WHERE LAST_SEEN >= UTC_TIMESTAMP() - INTERVAL ? SECOND
			AND SCHEMA_NAME IS NOT NULL
			AND SCHEMA_NAME NOT IN (?)
		ORDER BY avg_elapsed_time_ms DESC
		LIMIT ?;
    `

	/*
		CurrentRunningQueriesSearch: Fetches current running queries that match a specific digest.
		Useful for real-time monitoring of active query execution, enabling the identification
		of long-running queries that may need intervention to maintain system performance.

		Arguments:
		1. Digest (STRING): The digest of the query to search for.
		2. Minimum execution time in seconds (INT): The minimum execution time to filter queries.
		3. Limit (INT): The maximum number of results to return.
	*/
	CurrentRunningQueriesSearch = `
		SELECT
			DIGEST AS query_id,
			CASE
				WHEN CHAR_LENGTH(DIGEST_TEXT) > 4000 THEN CONCAT(LEFT(DIGEST_TEXT, 3997), '...')
				ELSE DIGEST_TEXT
			END AS query_text,
			SQL_TEXT AS query_sample_text,
			EVENT_ID AS event_id,
			THREAD_ID AS thread_id,
			ROUND(TIMER_WAIT / 1000000000, 3) AS execution_time_ms,
			ROWS_SENT AS rows_sent,
			ROWS_EXAMINED AS rows_examined,
			CURRENT_SCHEMA AS database_name
		FROM performance_schema.events_statements_current
		WHERE DIGEST = ?
			AND TIMER_WAIT / 1000000000 > ?
		ORDER BY TIMER_WAIT DESC
		LIMIT ?;
	`

	/*
		RecentQueriesSearch: Retrieves recent queries from history matching a specific digest.
		This query helps in assessing recently executed queries, reviewing their performance,
		and planning optimizations based on typical execution times and data handling patterns.

		Arguments:
		1. Digest (STRING): The digest of the query to search for.
		2. Minimum execution time in seconds (INT): The minimum execution time to filter queries.
		3. Limit (INT): The maximum number of results to return.
	*/
	RecentQueriesSearch = `
		SELECT
			DIGEST AS query_id,
			CASE
				WHEN CHAR_LENGTH(DIGEST_TEXT) > 4000 THEN CONCAT(LEFT(DIGEST_TEXT, 3997), '...')
				ELSE DIGEST_TEXT
			END AS query_text,
			SQL_TEXT AS query_sample_text,
			EVENT_ID AS event_id,
			THREAD_ID AS thread_id,
			ROUND(TIMER_WAIT / 1000000000, 3) AS execution_time_ms,
			ROWS_SENT AS rows_sent,
			ROWS_EXAMINED AS rows_examined,
			CURRENT_SCHEMA AS database_name
		FROM performance_schema.events_statements_history
		WHERE DIGEST = ?
			AND TIMER_WAIT / 1000000000 > ?
		ORDER BY TIMER_WAIT DESC
		LIMIT ?;
	`

	/*
		PastQueriesSearch: Fetches past long-running queries from the history long table based on a digest.
		This is useful for diagnosing historical performance issues and understanding query behavior over time.
		By examining past queries, you can discover inefficient patterns and possible optimization strategies.

		Arguments:
		1. Digest (STRING): The digest of the query to search for.
		2. Minimum execution time in seconds (INT): The minimum execution time to filter queries.
		3. Limit (INT): The maximum number of results to return.
	*/
	PastQueriesSearch = `
		SELECT
			DIGEST AS query_id,
			CASE
				WHEN CHAR_LENGTH(DIGEST_TEXT) > 4000 THEN CONCAT(LEFT(DIGEST_TEXT, 3997), '...')
				ELSE DIGEST_TEXT
			END AS query_text,
			SQL_TEXT AS query_sample_text,
			EVENT_ID AS event_id,
			THREAD_ID AS thread_id,
			ROUND(TIMER_WAIT / 1000000000, 3) AS execution_time_ms,
			ROWS_SENT AS rows_sent,
			ROWS_EXAMINED AS rows_examined,
			CURRENT_SCHEMA AS database_name
		FROM performance_schema.events_statements_history_long
		WHERE DIGEST = ?
			AND TIMER_WAIT / 1000000000 > ?
		ORDER BY TIMER_WAIT DESC
		LIMIT ?;
	`

	/*
		WaitEventsQuery: Analyzes waiting events across different sessions for query performance monitoring.
		This query collects wait event data which is crucial to understanding bottlenecks such as IO, locks,
		or synchronization issues. By categorizing wait events, it assists in diagnosing specific areas
		impacting database performance.

		Arguments:
		1. Excluded databases (STRING): A comma-separated list of database names to exclude from the results.
		2. Limit (INT): The maximum number of results to return.
	*/
	WaitEventsQuery = `
		WITH
		wait_data_aggregated AS (
			SELECT
				w.THREAD_ID,
				w.EVENT_NAME AS wait_event_name,
				SUM(w.TIMER_WAIT) AS total_wait_time,
				COUNT(*) AS wait_event_count
			FROM
				performance_schema.events_waits_current w
			GROUP BY
				w.THREAD_ID,
				w.EVENT_NAME
			UNION ALL
			SELECT
				w.THREAD_ID,
				w.EVENT_NAME AS wait_event_name,
				SUM(w.TIMER_WAIT) AS total_wait_time,
				COUNT(*) AS wait_event_count
			FROM
				performance_schema.events_waits_history w
			GROUP BY
				w.THREAD_ID,
				w.EVENT_NAME
		),
		schema_data AS (
			SELECT
				s.THREAD_ID,
				s.DIGEST AS query_id,
				s.CURRENT_SCHEMA AS database_name,
				s.DIGEST_TEXT AS query_text
			FROM
				performance_schema.events_statements_current s
			WHERE
				s.CURRENT_SCHEMA NOT IN (?)
			UNION ALL
			SELECT
				s.THREAD_ID,
				s.DIGEST AS query_id,
				s.CURRENT_SCHEMA AS database_name,
				s.DIGEST_TEXT AS query_text
			FROM
				performance_schema.events_statements_history s
			WHERE
				s.CURRENT_SCHEMA NOT IN (?)
		),
		joined_data AS (
			SELECT
				wda.wait_event_name,
				wda.total_wait_time,
				wda.wait_event_count,
				sd.query_id,
				sd.database_name,
				sd.query_text
			FROM
				wait_data_aggregated wda
			JOIN
				schema_data sd ON wda.THREAD_ID = sd.THREAD_ID
		)
		SELECT
			jd.query_id,
			jd.database_name,
			jd.wait_event_name,
			CASE
				WHEN jd.wait_event_name LIKE 'wait/io/file/innodb/%' THEN 'InnoDB File IO'
				WHEN jd.wait_event_name LIKE 'wait/io/file/sql/%' THEN 'SQL File IO'
				WHEN jd.wait_event_name LIKE 'wait/io/socket/%' THEN 'Network IO'
				WHEN jd.wait_event_name LIKE 'wait/synch/cond/%' THEN 'Condition Wait'
				WHEN jd.wait_event_name LIKE 'wait/synch/mutex/%' THEN 'Mutex'
				WHEN jd.wait_event_name LIKE 'wait/lock/table/%' THEN 'Table Lock'
				WHEN jd.wait_event_name LIKE 'wait/lock/metadata/%' THEN 'Metadata Lock'
				WHEN jd.wait_event_name LIKE 'wait/lock/transaction/%' THEN 'Transaction Lock'
				ELSE 'Other'
			END AS wait_category,
			ROUND(SUM(jd.total_wait_time) / 1000000000, 3) AS total_wait_time_ms,
			SUM(jd.wait_event_count) AS wait_event_count,
			ROUND(SUM(jd.total_wait_time) / 1000000000 / SUM(jd.wait_event_count), 3) AS avg_wait_time_ms,
			CASE
				WHEN CHAR_LENGTH(jd.query_text) > 4000 THEN CONCAT(LEFT(jd.query_text, 3997), '...')
				ELSE jd.query_text
			END AS query_text,
			DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
		FROM
			joined_data jd
		WHERE jd.query_id IS NOT NULL
		GROUP BY
			jd.query_id,
			jd.database_name,
			jd.wait_event_name,
			wait_category,
			jd.query_text
		ORDER BY
			total_wait_time_ms DESC
		LIMIT ?;
	`

	/*
		BlockingSessionsQuery: Identifies and details current database transactions that are blocked by others.
		This query provides information about blocked and blocking transactions, including their execution time
		and queries involved. It is vital for detecting deadlocks or contention issues, helping in resolving
		immediate blocking problems and planning long-term query and index optimizations to reduce this
		occurrence.

		Arguments:
		1. Excluded databases (STRING): A comma-separated list of database names to exclude from the results.
		2. Limit (INT): The maximum number of results to return.
	*/
	BlockingSessionsQuery = `
		SELECT 
                      r.trx_id AS blocked_txn_id,
                      r.trx_mysql_thread_id AS blocked_thread_id,
					  wt.PROCESSLIST_ID AS blocked_pid,
                      wt.PROCESSLIST_HOST AS blocked_host,
                      wt.PROCESSLIST_DB AS database_name,
					  wt.PROCESSLIST_STATE AS blocked_status,
                      b.trx_id AS blocking_txn_id,
                      b.trx_mysql_thread_id AS blocking_thread_id,
					  bt.PROCESSLIST_ID AS blocking_pid,
                      bt.PROCESSLIST_HOST AS blocking_host,
                      es_waiting.DIGEST_TEXT AS blocked_query,
                      es_blocking.DIGEST_TEXT AS blocking_query,
					  es_waiting.DIGEST AS blocked_query_id,
                      es_blocking.DIGEST AS blocking_query_id,
    				  bt.PROCESSLIST_STATE AS blocking_status,
					  ROUND(esc_waiting.TIMER_WAIT / 1000000000, 3) AS blocked_query_time_ms,
					  ROUND(esc_blocking.TIMER_WAIT / 1000000000, 3) AS blocking_query_time_ms,
					  DATE_FORMAT(CONVERT_TZ(r.trx_started, @@session.time_zone, '+00:00'), '%Y-%m-%dT%H:%i:%sZ') AS blocked_txn_start_time,
					  DATE_FORMAT(CONVERT_TZ(b.trx_started, @@session.time_zone, '+00:00'), '%Y-%m-%dT%H:%i:%sZ') AS blocking_txn_start_time,
					  DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
                  FROM 
                      performance_schema.data_lock_waits w
                  JOIN 
                      performance_schema.threads wt ON wt.THREAD_ID = w.REQUESTING_THREAD_ID
                  JOIN 
                      information_schema.innodb_trx r ON r.trx_mysql_thread_id = wt.PROCESSLIST_ID
                  JOIN 
                      performance_schema.threads bt ON bt.THREAD_ID = w.BLOCKING_THREAD_ID
                  JOIN 
                      information_schema.innodb_trx b ON b.trx_mysql_thread_id = bt.PROCESSLIST_ID
                  JOIN 
                      performance_schema.events_statements_current esc_waiting ON esc_waiting.THREAD_ID = wt.THREAD_ID
                  JOIN 
                      performance_schema.events_statements_summary_by_digest es_waiting 
                      ON esc_waiting.DIGEST = es_waiting.DIGEST
                  JOIN 
                      performance_schema.events_statements_current esc_blocking ON esc_blocking.THREAD_ID = bt.THREAD_ID
                  JOIN 
                      performance_schema.events_statements_summary_by_digest es_blocking 
                      ON esc_blocking.DIGEST = es_blocking.DIGEST
				  WHERE
					  wt.PROCESSLIST_DB IS NOT NULL
					  AND wt.PROCESSLIST_DB NOT IN (?)
				  ORDER BY 
					  blocked_txn_start_time ASC
				  LIMIT ?;
	`
)
