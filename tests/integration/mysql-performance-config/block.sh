# Execute blocking session queries
# Start a new tmux session named 'mysession'
tmux new-session -d -s mysql_block_test
echo "started executing blocking session queries"
# First window
tmux send-keys -t mysql_block_test:0 "docker exec -i mysql_8-0-40 mysql -u root -pDBpwd1234 -e \"
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
USE employees;
START TRANSACTION;
UPDATE employees SET last_name = 'Blocking' WHERE emp_no = 10001;
SELECT SLEEP(300);\"" C-m

tmux split-window -t mysql_block_test:0

tmux send-keys -t mysql_block_test:0.1 "docker exec -i mysql_8-0-40 mysql -u root -pDBpwd1234 -e \"
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
USE employees;
START TRANSACTION;
UPDATE employees SET last_name = 'Blocked' WHERE emp_no = 10001;
SELECT SLEEP(30000);\"" C-m
echo "finished executing blocking session queries"

tmux attach-session -t mysql_block_test