// +build integration

package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-mysql/tests/integration/helpers"
	"github.com/newrelic/nri-mysql/tests/integration/jsonschema"
)

var (
	iName = "mysql"

	defaultContainer = "integration_nri-mysql_1"
	// mysql config
	defaultBinPath         = "/nri-mysql"
	defaultMysqlUser       = "root"
	defaultMysqlPass       = "DBpwd1234"
	defaultMysqlMasterHost = "mysql_master"
	defaultMysqlSlaveHost  = "mysql_slave"
	defaultMysqlPort       = 3306
	defaultMysqlDB         = "database"

	// cli flags
	container  = flag.String("container", defaultContainer, "container where the integration is installed")
	binPath    = flag.String("bin", defaultBinPath, "Integration binary path")
	user       = flag.String("user", defaultMysqlUser, "Mysql user name")
	psw        = flag.String("psw", defaultMysqlPass, "Mysql user password")
	masterHost = flag.String("masterhost", defaultMysqlMasterHost, "Mysql master host ip address")
	slaveHost  = flag.String("slavehost", defaultMysqlSlaveHost, "Mysql master host ip address")
	port       = flag.Int("port", defaultMysqlPort, "Mysql port")
	database   = flag.String("database", defaultMysqlDB, "Mysql database")
)

// Returns the standard output, or fails testing if the command returned an error
func runIntegration(t *testing.T, targetContainer string, envVars ...string) string {
	t.Helper()

	command := make([]string, 0)
	command = append(command, *binPath)
	if user != nil {
		command = append(command, "--username", *user)
	}
	if psw != nil {
		command = append(command, "--password", *psw)
	}
	if targetContainer != "" {
		command = append(command, "--hostname", targetContainer)
	}
	if port != nil {
		command = append(command, "--port", strconv.Itoa(*port))
	}
	if database != nil {
		command = append(command, "--database", *database)
	}
	stdout, stderr, err := helpers.ExecInContainer(*container, command, envVars...)
	if stderr != "" {
		log.Debug("Integration command Standard Error: ", stderr)
	}
	require.NoError(t, err)

	return stdout
}

func setup() error {
	flag.Parse()

	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	masterErr := helpers.WaitForPort(*container, *masterHost, *port, 60*time.Second)
	if masterErr != nil {
		return masterErr
	}

	slaveErr := helpers.WaitForPort(*container, *slaveHost, *port, 30*time.Second)
	if slaveErr != nil {
		return slaveErr
	}

	// Retrieve log filename and position from master
	masterStatusCmd := []string{`mysql`, `-u`, `root`, `-e`, `SHOW MASTER STATUS;`}
	masterStatusOut, masterStatusErr, err := helpers.ExecInContainer(*masterHost, masterStatusCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
	if masterStatusErr != "" {
		log.Debug("Error fetching Master Log filename and Position: ", masterStatusErr)
		return err
	}

	masterStatus := strings.Fields(masterStatusOut)
	masterLogFile := masterStatus[5]
	masterLogPos := masterStatus[6]

	// Activate MASTER/SLAVE replication
	replication_stmt := fmt.Sprintf(`CHANGE MASTER TO MASTER_HOST='%s', MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_LOG_FILE='%s', MASTER_LOG_POS=%v; START SLAVE;`, *masterHost, *user, *psw, masterLogFile, masterLogPos)
	replicationCmd := []string{`mysql`, `-u`, `root`, `-e`, replication_stmt}
	_, replicationStatusErr, err := helpers.ExecInContainer(*slaveHost, replicationCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
	if replicationStatusErr != "" {
		log.Debug("Error creating Master/Slave replication: ", replicationStatusErr)
		return err
	}
	log.Info("Setup Complete!")

	return nil
}

func teardown() error {
	return nil
}

func TestMain(m *testing.M) {
	err := setup()
	if err != nil {
		fmt.Println(err)
		tErr := teardown()
		if tErr != nil {
			fmt.Printf("Error during the teardown of the tests: %s\n", tErr)
		}
		os.Exit(1)
	}

	result := m.Run()

	err = teardown()
	if err != nil {
		fmt.Printf("Error during the teardown of the tests: %s\n", err)
	}

	os.Exit(result)
}

func TestOutputIsValidJSON(t *testing.T) {
	stdout := runIntegration(t, *masterHost)

	var j map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestMySQLIntegrationValidArguments_RemoteEntity(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, *masterHost, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName), "REMOTE_MONITORING=true")
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationValidArguments_LocalEntity(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, *masterHost, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-master-localentity.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationOnlyMetrics(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, *masterHost, "METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-metrics-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlyInventory(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, *masterHost, "INTEGRATION=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-inventory-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlySlaveMetrics(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, *slaveHost, "METRICS=true", "EXTENDED_METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-metrics-slave.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}
