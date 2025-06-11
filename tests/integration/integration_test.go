//go:build integration

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
	defaultBinPath   = "/nri-mysql"
	defaultMysqlUser = "root"
	defaultMysqlPass = "DBpwd1234"
	defaultMysqlPort = 3306
	defaultMysqlDB   = "database"

	// cli flags
	container = flag.String("container", defaultContainer, "container where the integration is installed")
	binPath   = flag.String("bin", defaultBinPath, "Integration binary path")
	user      = flag.String("user", defaultMysqlUser, "Mysql user name")
	psw       = flag.String("psw", defaultMysqlPass, "Mysql user password")
	port      = flag.Int("port", defaultMysqlPort, "Mysql port")
	database  = flag.String("database", defaultMysqlDB, "Mysql database")
)

type MysqlConfig struct {
	Version        string // Mysql server version
	MasterHostname string // MasterHostname for the Mysql service. (Will be the master mysql service inside the docker-compose file).
	SlaveHostname  string // SlaveHostname for the Mysql service. (Will be the slave mysql service inside the docker-compose file).
}

var (
	MysqlConfigs = []MysqlConfig{
		{
			Version:        "5.7.35",
			MasterHostname: "mysql_master-5-7-35",
			SlaveHostname:  "mysql_slave-5-7-35",
		},
		{
			/*
				The query cache variables are removed from MySQL 8.0 - https://dev.mysql.com/doc/refman/5.7/en/query-cache-status-and-maintenance.html
				Due to which the all qcache metrics are not supported from this version

				From MySQL 8.0.23 the statement CHANGE MASTER TO is deprecated. The alias CHANGE REPLICATION SOURCE TO should be used instead.
				The parameters for the statement also have aliases that replace the term MASTER with the term SOURCE.
				For example, MASTER_HOST and MASTER_PORT can now be entered as SOURCE_HOST and SOURCE_PORT.
				More Info - https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-23.html
			*/
			Version:        "8.0.40",
			MasterHostname: "mysql_master-8-0-40",
			SlaveHostname:  "mysql_slave-8-0-40",
		},
		{
			Version:        "9.1.0",
			MasterHostname: "mysql_master-latest-supported",
			SlaveHostname:  "mysql_slave-latest-supported",
		},
	}
)

// Returns the standard output, or fails testing if the command returned an error
func runIntegration(t *testing.T, targetContainer string, envVars ...string) string {
	stdout, stderr, err := helpers.RunIntegrationAndGetStdout(t, binPath, user, psw, port, nil, nil, container, targetContainer, envVars)
	if stderr != "" {
		log.Debug("Integration command Standard Error: ", stderr)
	}
	require.NoError(t, err)

	return stdout
}

func runIntegrationAndGetStdoutWithError(t *testing.T, targetContainer string, envVars ...string) (string, string, error) {
	return helpers.RunIntegrationAndGetStdout(t, binPath, user, psw, port, nil, nil, container, targetContainer, envVars)
}

func checkVersion(dbVersion string) bool {
	parts := strings.Split(dbVersion, ".")

	majorVersion, err1 := strconv.Atoi(parts[0])
	minorVersion, err2 := strconv.Atoi(parts[1])

	if err1 != nil || err2 != nil {
		return false
	}

	if majorVersion == 8 {
		if minorVersion >= 4 {
			return true
		} else {
			return false
		}
	} else if majorVersion > 8 {
		return true
	}
	return false
}

func isDBVersionLessThan8(dbVersion string) bool {
	parts := strings.Split(dbVersion, ".")

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return true
	}

	return majorVersion < 8
}

func setup(mysqlConfig MysqlConfig) error {
	flag.Parse()

	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	masterErr := helpers.WaitForPort(*container, mysqlConfig.MasterHostname, *port, 60*time.Second)
	if masterErr != nil {
		return masterErr
	}

	slaveErr := helpers.WaitForPort(*container, mysqlConfig.SlaveHostname, *port, 30*time.Second)
	if slaveErr != nil {
		return slaveErr
	}

	// Retrieve log filename and position from master
	var masterStatusQuery = ""
	if checkVersion(mysqlConfig.Version) {
		masterStatusQuery = `SHOW BINARY LOG STATUS;`
	} else {
		masterStatusQuery = `SHOW MASTER STATUS;`
	}
	masterStatusCmd := []string{`mysql`, `-u`, `root`, `-e`, masterStatusQuery}
	masterStatusOut, masterStatusErr, err := helpers.ExecInContainer(mysqlConfig.MasterHostname, masterStatusCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
	if masterStatusErr != "" {
		log.Debug("Error fetching Master Log filename and Position: ", masterStatusErr)
		return err
	}

	masterStatus := strings.Fields(masterStatusOut)
	masterLogFile := masterStatus[5]
	masterLogPos := masterStatus[6]

	// Activate MASTER/SLAVE replication
	var replication_stmt = ""
	if isDBVersionLessThan8(mysqlConfig.Version) {
		replication_stmt = fmt.Sprintf(`CHANGE MASTER TO MASTER_HOST='%s', MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_LOG_FILE='%s', MASTER_LOG_POS=%v; START SLAVE;`, mysqlConfig.MasterHostname, *user, *psw, masterLogFile, masterLogPos)
	} else {
		replication_stmt = fmt.Sprintf(`CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_USER='%s', SOURCE_PASSWORD='%s', SOURCE_LOG_FILE='%s', SOURCE_LOG_POS=%v; START REPLICA; GRANT ALL ON *.* TO %s;`, mysqlConfig.MasterHostname, *user, *psw, masterLogFile, masterLogPos, *user)
	}
	replicationCmd := []string{`mysql`, `-u`, `root`, `-e`, replication_stmt}
	_, replicationStatusErr, err := helpers.ExecInContainer(mysqlConfig.SlaveHostname, replicationCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
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
	for _, mysqlConfig := range MysqlConfigs {
		err := setup(mysqlConfig)
		if err != nil {
			fmt.Println(err)
			tErr := teardown()
			if tErr != nil {
				fmt.Printf("Error during the teardown of the tests: %s\n", tErr)
			}
			os.Exit(1)
		}
	}

	result := m.Run()

	err := teardown()
	if err != nil {
		fmt.Printf("Error during the teardown of the tests: %s\n", err)
	}

	os.Exit(result)
}

func testOutputIsValidJSON(t *testing.T, mysqlConfig MysqlConfig) {
	stdout := runIntegration(t, mysqlConfig.MasterHostname)
	var j map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestOutputIsValidJSON(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testOutputIsValidJSON(t, mysqlConfig)
	}
}

func testMySQLIntegrationValidArguments_RemoteEntity(t *testing.T, mysqlConfig MysqlConfig) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, mysqlConfig.MasterHostname, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName), "REMOTE_MONITORING=true")
	schemaDir := fmt.Sprintf("json-schema-files-%s", mysqlConfig.Version)
	schemaPath := filepath.Join(schemaDir, "mysql-schema-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationValidArguments_RemoteEntity(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testMySQLIntegrationValidArguments_RemoteEntity(t, mysqlConfig)
	}
}

func testMySQLIntegrationValidArguments_LocalEntity(t *testing.T, mysqlConfig MysqlConfig) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, mysqlConfig.MasterHostname, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaDir := fmt.Sprintf("json-schema-files-%s", mysqlConfig.Version)
	schemaPath := filepath.Join(schemaDir, "mysql-schema-master-localentity.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationValidArguments_LocalEntity(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testMySQLIntegrationValidArguments_LocalEntity(t, mysqlConfig)
	}
}

func testMySQLIntegrationOnlyMetrics(t *testing.T, mysqlConfig MysqlConfig) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, mysqlConfig.MasterHostname, "METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaDir := fmt.Sprintf("json-schema-files-%s", mysqlConfig.Version)
	schemaPath := filepath.Join(schemaDir, "mysql-schema-metrics-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlyMetrics(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testMySQLIntegrationOnlyMetrics(t, mysqlConfig)
	}
}

func testMySQLIntegrationOnlyInventory(t *testing.T, mysqlConfig MysqlConfig) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, mysqlConfig.MasterHostname, "INTEGRATION=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaDir := fmt.Sprintf("json-schema-files-%s", mysqlConfig.Version)
	schemaPath := filepath.Join(schemaDir, "mysql-schema-inventory-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlyInventory(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testMySQLIntegrationOnlyInventory(t, mysqlConfig)
	}
}

func testMySQLIntegrationOnlySlaveMetrics(t *testing.T, mysqlConfig MysqlConfig) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, mysqlConfig.SlaveHostname, "METRICS=true", "EXTENDED_METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaDir := fmt.Sprintf("json-schema-files-%s", mysqlConfig.Version)
	schemaPath := filepath.Join(schemaDir, "mysql-schema-metrics-slave.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlySlaveMetrics(t *testing.T) {
	for _, mysqlConfig := range MysqlConfigs {
		testMySQLIntegrationOnlySlaveMetrics(t, mysqlConfig)
	}
}

func runUnconfiguredMysqlPerfConfigTest(t *testing.T, args []string, outputMetricsFile string, expectedError string, testName string) {
	for _, mysqlUnconfiguredPerfConfig := range MysqlConfigs {
		if isDBVersionLessThan8(mysqlUnconfiguredPerfConfig.Version) {
			// performance metrics are supported for mysql version 8 and above
			// so, skipping if the mysql version is less than 8
			continue
		}
		t.Run(testName+mysqlUnconfiguredPerfConfig.Version, func(t *testing.T) {
			args = append(args, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
			stdout, stderr, err := runIntegrationAndGetStdoutWithError(t, mysqlUnconfiguredPerfConfig.MasterHostname, args...)
			outputMetricsList := strings.Split(stdout, "\n")
			if len(outputMetricsList) > 1 {
				assert.Empty(t, outputMetricsList[1], "Unexpected stdout content")
			}
			helpers.AssertReceivedErrors(t, expectedError, strings.Split(stderr, "\n")...)

			// For QueryMonitoringOnly case, we don't need to validate any output
			if strings.Contains(testName, "QueryMonitoringOnly") {
				t.Logf("Skipping schema validation for QueryMonitoringOnly - no validation required")
				return
			}

			// Skip schema validation if there's no output to validate
			if len(outputMetricsList) == 0 || (len(outputMetricsList) == 1 && strings.TrimSpace(outputMetricsList[0]) == "") {
				t.Logf("Empty output - skipping schema validation")
				return
			}

			schemaPath := filepath.Join("json-schema-performance-files", outputMetricsFile)
			err = jsonschema.Validate(schemaPath, outputMetricsList[0])
			require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
		})
	}
}

// Run integration with ENABLE_QUERY_MONITORING flag enabled for mysql servers which don't have performance flags/extensions enabled
func TestUnconfiguredPerfMySQLIntegration(t *testing.T) {
	testCases := []struct {
		name              string
		args              []string
		outputMetricsFile string
		expectedError     string
	}{
		{
			name: "RemoteEntity_EnableQueryMonitoring",
			args: []string{
				"REMOTE_MONITORING=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-master.json",
			// We don't get any error in this case because the root user is being used to enable essential consumers and instruments via explicit queries.
			expectedError: "",
		},
		{
			name: "LocalEntity_EnableQueryMonitoring",
			args: []string{
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-master-localentity.json",
			// We don't get any error in this case because the root user is being used to enable essential consumers and instruments via explicit queries.
			expectedError: "",
		},
		{
			name: "OnlyMetrics_EnableQueryMonitoring",
			args: []string{
				"METRICS=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-metrics-master.json",
			// We don't get any error in this case because the root user is being used to enable essential consumers and instruments via explicit queries.
			expectedError: "",
		},
		{
			name: "OnlyInventory_EnableQueryMonitoring",
			args: []string{
				"INVENTORY=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-inventory-master.json",
			/*
				 	Note: Expected error is empty as integration will report query performance monitoring data when both metrics and enable_query_monitoring are enabled.
					Refer args.HasMetrics() implementation here https://github.com/newrelic/infra-integrations-sdk/blob/12ee4e8a20a479f2b3d9ba328d2f80c9dc663c79/args/args.go#L33
			*/
			expectedError: "",
		},
	}
	for _, testCase := range testCases {
		runUnconfiguredMysqlPerfConfigTest(t, testCase.args, testCase.outputMetricsFile, testCase.expectedError, testCase.name)
	}
}
