package utils

const (
	/*
		SlowQueries: Retrieves a list of slow queries that have been executed within a certain period.
		This query provides insights into slow-performing queries by capturing metrics such as average CPU time,
		average elapsed time, and disk reads/writes. It's beneficial for identifying resource-intensive queries
		that may require optimization, such as indexing or query restructuring.
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
			AND DIGEST_TEXT RLIKE '^(SELECT|INSERT|UPDATE|DELETE|WITH)'
			AND DIGEST_TEXT NOT LIKE '%DIGEST_TEXT%'
		ORDER BY avg_elapsed_time_ms DESC
		LIMIT ?;
    `

	/*
		CurrentRunningQueriesSearch: Fetches current running queries that match a specific digest.
		Useful for real-time monitoring of active query execution, enabling the identification
		of long-running queries that may need intervention to maintain system performance.
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
	*/
	WaitEventsQuery = `
		WITH wait_data AS (
			SELECT DISTINCT
				THREAD_ID,
				OBJECT_INSTANCE_BEGIN AS instance_id,
				EVENT_NAME AS wait_event_name,
				TIMER_WAIT,
				TIMER_START
			FROM performance_schema.events_waits_current
			UNION ALL
			SELECT DISTINCT
				THREAD_ID,
				OBJECT_INSTANCE_BEGIN AS instance_id,
				EVENT_NAME AS wait_event_name,
				TIMER_WAIT,
				TIMER_START
			FROM performance_schema.events_waits_history
		),
		schema_data AS (
			SELECT DISTINCT
				THREAD_ID,
				DIGEST,
				CURRENT_SCHEMA AS database_name,
				DIGEST_TEXT AS query_text,
				ROUND(TIMER_WAIT / 1000000000, 3) AS execution_time_ms
			FROM performance_schema.events_statements_current
			WHERE CURRENT_SCHEMA NOT IN (?)
			AND SQL_TEXT RLIKE '^(SELECT|INSERT|UPDATE|DELETE|WITH)'
			AND DIGEST_TEXT NOT LIKE '%DIGEST_TEXT%'
			UNION ALL
			SELECT DISTINCT
				THREAD_ID,
				DIGEST,
				CURRENT_SCHEMA AS database_name,
				DIGEST_TEXT AS query_text,
				ROUND(TIMER_WAIT / 1000000000, 3) AS execution_time_ms
			FROM performance_schema.events_statements_history
			WHERE CURRENT_SCHEMA NOT IN (?)
			AND SQL_TEXT RLIKE '^(SELECT|INSERT|UPDATE|DELETE|WITH)'
			AND DIGEST_TEXT NOT LIKE '%DIGEST_TEXT%'
		)
		SELECT
			schema_data.DIGEST AS query_id,
			wait_data.instance_id,
			schema_data.database_name,
			wait_data.wait_event_name,
			CASE
				WHEN wait_data.wait_event_name LIKE 'wait/io/file/innodb/%' THEN 'InnoDB File IO'
				WHEN wait_data.wait_event_name LIKE 'wait/io/file/sql/%' THEN 'SQL File IO'
				WHEN wait_data.wait_event_name LIKE 'wait/io/socket/%' THEN 'Network IO'
				WHEN wait_data.wait_event_name LIKE 'wait/synch/cond/%' THEN 'Condition Wait'
				WHEN wait_data.wait_event_name LIKE 'wait/synch/mutex/%' THEN 'Mutex'
				WHEN wait_data.wait_event_name LIKE 'wait/lock/table/%' THEN 'Table Lock'
				WHEN wait_data.wait_event_name LIKE 'wait/lock/metadata/%' THEN 'Metadata Lock'
				WHEN wait_data.wait_event_name LIKE 'wait/lock/transaction/%' THEN 'Transaction Lock'
				ELSE 'Other'
			END AS wait_category,
			ROUND(SUM(wait_data.TIMER_WAIT) / 1000000000, 3) AS total_wait_time_ms,
			COUNT(DISTINCT wait_data.instance_id) AS wait_event_count,
			ROUND(SUM(wait_data.TIMER_WAIT) / 1000000000 / COUNT(DISTINCT wait_data.instance_id), 3) AS avg_wait_time_ms,
			CASE
				WHEN CHAR_LENGTH(schema_data.query_text) > 4000 THEN CONCAT(LEFT(schema_data.query_text, 3997), '...')
				ELSE schema_data.query_text
			END AS query_text,
			DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-%dT%H:%i:%sZ') AS collection_timestamp
		FROM wait_data
		JOIN schema_data ON wait_data.THREAD_ID = schema_data.THREAD_ID
		GROUP BY
			query_id,
			wait_data.instance_id,
			schema_data.database_name,
			wait_data.wait_event_name,
			wait_category,
			schema_data.query_text
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
