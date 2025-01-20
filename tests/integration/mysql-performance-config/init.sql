-- Enable required Performance Schema instruments
UPDATE performance_schema.setup_instruments
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE 'wait/%';

-- Enable CPU Instruments
UPDATE performance_schema.setup_instruments
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE 'statement/%';

-- Enable Collection of Current Data Lock Waits
UPDATE performance_schema.setup_instruments
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE '%lock%';

-- Enable required Performance Schema consumers
UPDATE performance_schema.setup_consumers
SET ENABLED = 'YES'
WHERE NAME IN ('events_waits_current', 'events_waits_history_long', 'events_waits_history', 'events_statements_history_long');

-- Enable required Performance Schema consumers for CPU metrics
UPDATE performance_schema.setup_consumers
SET ENABLED = 'YES'
WHERE NAME IN ('events_statements_history', 'events_statements_current', 'events_statements_cpu');

UPDATE performance_schema.setup_consumers
SET ENABLED = 'YES'
WHERE NAME LIKE 'events_waits_current%' OR NAME LIKE 'events_transactions_current%' OR NAME LIKE 'events_statements_current%' OR NAME LIKE 'events_stages_current%';

SET GLOBAL innodb_lock_wait_timeout = 120; -- Increase to 2 minutes