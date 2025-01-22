#!/bin/bash
set -x

usermod -d /var/lib/mysql/ mysql

# Run the MySQL default entrypoint setup
/usr/local/bin/docker-entrypoint.sh mysqld &

# Wait for MySQL to be fully initialized and ready
until mysqladmin ping --silent; do
    echo "Waiting for MySQL server..."
    sleep 2
done

# Populated Employees Sample Data
echo "started copying sample employees data"

mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -t < employees.sql

echo "completed copying sample employees data"

sleep 10

# Execute slow queries
echo "started executing slow queries"
mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -e "
USE employees;
SELECT e.emp_no, e.first_name, e.last_name
FROM employees e
WHERE EXISTS (
    SELECT 1
    FROM salaries s
    WHERE s.emp_no = e.emp_no
    AND s.salary > (
        SELECT AVG(salary)
        FROM salaries
        WHERE to_date = '9999-01-01'
    )
);
"

sleep 5

mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -e "
USE employees;
SELECT e.emp_no, e.first_name, e.last_name, 
       (SELECT COUNT(*) FROM dept_emp de WHERE de.emp_no = e.emp_no) AS dept_count,
       (SELECT AVG(salary) FROM salaries s WHERE s.emp_no = e.emp_no AND s.to_date = '9999-01-01') AS avg_salary
FROM employees e
ORDER BY avg_salary DESC;
"
echo "finshed executing slow queries"

# Execute blocking session queries

sleep 5

echo "started executing blocking session query 1"
mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -e "
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
USE employees;
START TRANSACTION;
UPDATE employees SET last_name = 'Blocking' WHERE emp_no = 10001;
SELECT SLEEP(10);"
echo "finished executing blocking session query 1"

sleep 5
# Start a new tmux session named 'mysession'
tmux new-session -d -s mysql_block_test
# First window

echo "started executing blocking session query 2"
tmux send-keys -t mysql_block_test:0 "mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -e \"
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
USE employees;
START TRANSACTION;
UPDATE employees SET last_name = 'Blocked' WHERE emp_no = 10001;
SELECT SLEEP(3000);\"" C-m
echo "finished executing blocking session query 2"

sleep 5

tmux split-window -t mysql_block_test:0

tmux send-keys -t mysql_block_test:0.1 "docker exec -i mysql_8-0-40 mysql --user=root --password="${MYSQL_ROOT_PASSWORD}" -e \"
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
USE employees;
START TRANSACTION;
UPDATE employees SET last_name = 'Blocked-2' WHERE emp_no = 10001;
SELECT SLEEP(300);\"" C-m
echo "finished executing blocking session queries"

# Handle foreground execution to keep the container running
wait