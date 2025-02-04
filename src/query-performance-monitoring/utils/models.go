package utils

type SlowQueryMetrics struct {
	QueryID                *string  `json:"query_id" db:"query_id" metric_name:"query_id" source_type:"attribute"`
	QueryText              *string  `json:"query_text" db:"query_text" metric_name:"query_text" source_type:"attribute"`
	DatabaseName           *string  `json:"database_name" db:"database_name" metric_name:"database_name" source_type:"attribute"`
	SchemaName             *string  `json:"schema_name" db:"schema_name" metric_name:"schema_name" source_type:"attribute"`
	ExecutionCount         *uint64  `json:"execution_count" db:"execution_count" metric_name:"execution_count" source_type:"gauge"`
	AvgCPUTimeMs           *float64 `json:"avg_cpu_time_ms" db:"avg_cpu_time_ms" metric_name:"avg_cpu_time_ms" source_type:"gauge"`
	AvgElapsedTimeMs       *float64 `json:"avg_elapsed_time_ms" db:"avg_elapsed_time_ms" metric_name:"avg_elapsed_time_ms" source_type:"gauge"`
	AvgDiskReads           *float64 `json:"avg_disk_reads" db:"avg_disk_reads" metric_name:"avg_disk_reads" source_type:"gauge"`
	AvgDiskWrites          *float64 `json:"avg_disk_writes" db:"avg_disk_writes" metric_name:"avg_disk_writes" source_type:"gauge"`
	HasFullTableScan       *string  `json:"has_full_table_scan" db:"has_full_table_scan" metric_name:"has_full_table_scan" source_type:"attribute"`
	StatementType          *string  `json:"statement_type" db:"statement_type" metric_name:"statement_type" source_type:"attribute"`
	LastExecutionTimestamp *string  `json:"last_execution_timestamp" db:"last_execution_timestamp" metric_name:"last_execution_timestamp" source_type:"attribute"`
	CollectionTimestamp    *string  `json:"collection_timestamp" db:"collection_timestamp" metric_name:"collection_timestamp" source_type:"attribute"`
}

type IndividualQueryMetrics struct {
	QueryID             *string `json:"query_id" db:"query_id" metric_name:"query_id" source_type:"attribute"`
	AnonymizedQueryText *string `json:"query_text" db:"query_text" metric_name:"query_text" source_type:"attribute"`
	// QueryText is used only for fetching query execution plan and not ingested to New Relic
	QueryText       *string  `json:"query_sample_text" db:"query_sample_text" metric_name:"query_sample_text" source_type:"attribute"`
	EventID         *uint64  `json:"event_id" db:"event_id" metric_name:"event_id" source_type:"gauge"`
	ThreadID        *uint64  `json:"thread_id" db:"thread_id" metric_name:"thread_id" source_type:"gauge"`
	ExecutionTimeMs *float64 `json:"execution_time_ms" db:"execution_time_ms" metric_name:"execution_time_ms" source_type:"gauge"`
	RowsSent        *int64   `json:"rows_sent" db:"rows_sent" metric_name:"rows_sent" source_type:"gauge"`
	RowsExamined    *int64   `json:"rows_examined" db:"rows_examined" metric_name:"rows_examined" source_type:"gauge"`
	DatabaseName    *string  `json:"database_name" db:"database_name" metric_name:"database_name" source_type:"attribute"`
}

type QueryGroup struct {
	Database string
	Queries  []IndividualQueryMetrics
}

type QueryPlanMetrics struct {
	EventID             uint64 `json:"event_id" metric_name:"event_id" source_type:"gauge"`
	ThreadID            uint64 `json:"thread_id" db:"thread_id" metric_name:"thread_id" source_type:"gauge"`
	StepID              int    `json:"step_id" metric_name:"step_id" source_type:"gauge"`
	QueryCost           string `json:"query_cost" metric_name:"query_cost" source_type:"attribute"`
	TableName           string `json:"table_name" metric_name:"table_name" source_type:"attribute"`
	AccessType          string `json:"access_type" metric_name:"access_type" source_type:"attribute"`
	RowsExaminedPerScan int64  `json:"rows_examined_per_scan" metric_name:"rows_examined_per_scan" source_type:"gauge"`
	RowsProducedPerJoin int64  `json:"rows_produced_per_join" metric_name:"rows_produced_per_join" source_type:"gauge"`
	Filtered            string `json:"filtered" metric_name:"filtered" source_type:"attribute"`
	ReadCost            string `json:"read_cost" metric_name:"read_cost" source_type:"attribute"`
	EvalCost            string `json:"eval_cost" metric_name:"eval_cost" source_type:"attribute"`
	PossibleKeys        string `json:"possible_keys" metric_name:"possible_keys" source_type:"attribute"`
	Key                 string `json:"key" metric_name:"key" source_type:"attribute"`
	UsedKeyParts        string `json:"used_key_parts" metric_name:"used_key_parts" source_type:"attribute"`
	Ref                 string `json:"ref" metric_name:"ref" source_type:"attribute"`
	PrefixCost          string `json:"prefix_cost" metric_name:"prefix_cost" source_type:"attribute"`
	DataReadPerJoin     string `json:"data_read_per_join" metric_name:"data_read_per_join" source_type:"attribute"`
	UsingIndex          string `json:"using_index" metric_name:"using_index" source_type:"attribute"`
	KeyLength           string `json:"key_length" metric_name:"key_length" source_type:"attribute"`
}

type Memo struct {
	QueryCost string `json:"query_cost" metric_name:"query_cost" source_type:"gauge"`
}

type WaitEventQueryMetrics struct {
	TotalWaitTimeMs     *float64 `json:"total_wait_time_ms" db:"total_wait_time_ms" metric_name:"total_wait_time_ms" source_type:"gauge"`
	QueryID             *string  `json:"query_id" db:"query_id" metric_name:"query_id" source_type:"attribute"`
	QueryText           *string  `json:"query_text" db:"query_text" metric_name:"query_text" source_type:"attribute"`
	DatabaseName        *string  `json:"database_name" db:"database_name" metric_name:"database_name" source_type:"attribute"`
	WaitCategory        *string  `json:"wait_category" db:"wait_category" metric_name:"wait_category" source_type:"attribute"`
	CollectionTimestamp *string  `json:"collection_timestamp" db:"collection_timestamp" metric_name:"collection_timestamp" source_type:"attribute"`
	WaitEventName       *string  `json:"wait_event_name" db:"wait_event_name" metric_name:"wait_event_name" source_type:"attribute"`
	WaitEventCount      *uint64  `json:"wait_event_count" db:"wait_event_count" metric_name:"wait_event_count" source_type:"gauge"`
	AvgWaitTimeMs       *string  `json:"avg_wait_time_ms" db:"avg_wait_time_ms" metric_name:"avg_wait_time_ms" source_type:"attribute"`
}

type BlockingSessionMetrics struct {
	BlockedTxnID         *string  `json:"blocked_txn_id" db:"blocked_txn_id" metric_name:"blocked_txn_id" source_type:"attribute"`
	BlockedPID           *string  `json:"blocked_pid" db:"blocked_pid" metric_name:"blocked_pid" source_type:"attribute"`
	BlockedThreadID      *int64   `json:"blocked_thread_id" db:"blocked_thread_id" metric_name:"blocked_thread_id" source_type:"gauge"`
	BlockedQueryID       *string  `json:"blocked_query_id" db:"blocked_query_id" metric_name:"blocked_query_id" source_type:"attribute"`
	BlockedQuery         *string  `json:"blocked_query" db:"blocked_query" metric_name:"blocked_query" source_type:"attribute"`
	BlockedStatus        *string  `json:"blocked_status" db:"blocked_status" metric_name:"blocked_status" source_type:"attribute"`
	BlockedHost          *string  `json:"blocked_host" db:"blocked_host" metric_name:"blocked_host" source_type:"attribute"`
	BlockedDB            *string  `json:"database_name" db:"database_name" metric_name:"database_name" source_type:"attribute"`
	BlockingTxnID        *string  `json:"blocking_txn_id" db:"blocking_txn_id" metric_name:"blocking_txn_id" source_type:"attribute"`
	BlockingPID          *string  `json:"blocking_pid" db:"blocking_pid" metric_name:"blocking_pid" source_type:"attribute"`
	BlockingThreadID     *int64   `json:"blocking_thread_id" db:"blocking_thread_id" metric_name:"blocking_thread_id" source_type:"gauge"`
	BlockingHost         *string  `json:"blocking_host" db:"blocking_host" metric_name:"blocking_host" source_type:"attribute"`
	BlockingQueryID      *string  `json:"blocking_query_id" db:"blocking_query_id" metric_name:"blocking_query_id" source_type:"attribute"`
	BlockingQuery        *string  `json:"blocking_query" db:"blocking_query" metric_name:"blocking_query" source_type:"attribute"`
	BlockingStatus       *string  `json:"blocking_status" db:"blocking_status" metric_name:"blocking_status" source_type:"attribute"`
	BlockedQueryTimeMs   *float64 `json:"blocked_query_time_ms" db:"blocked_query_time_ms" metric_name:"blocked_query_time_ms" source_type:"gauge"`
	BlockingQueryTimeMs  *float64 `json:"blocking_query_time_ms" db:"blocking_query_time_ms" metric_name:"blocking_query_time_ms" source_type:"gauge"`
	BlockedTxnStartTime  *string  `json:"blocked_txn_start_time" db:"blocked_txn_start_time" metric_name:"blocked_txn_start_time" source_type:"attribute"`
	BlockingTxnStartTime *string  `json:"blocking_txn_start_time" db:"blocking_txn_start_time" metric_name:"blocking_txn_start_time" source_type:"attribute"`
	CollectionTimestamp  *string  `json:"collection_timestamp" db:"collection_timestamp" metric_name:"collection_timestamp" source_type:"attribute"`
}
