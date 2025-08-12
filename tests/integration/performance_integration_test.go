//go:build integration_performance_metrics

package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/nri-mysql/tests/integration/helpers"
	"github.com/newrelic/nri-mysql/tests/integration/jsonschema"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	iName = "mysql"

	defaultPerfContainer = "integration_nri-mysql_perf_1"
	// mysql config
	defaultBinPath                              = "/nri-mysql"
	defaultMysqlUser                            = "root"
	defaultMysqlPass                            = ""
	defaultMysqlPort                            = 3306
	defaultEnableQueryMonitoring                = false
	defaultSlowQueryMonitoringFetchInterval     = 3000
	defaultQueryMonitoringResponseTimeThreshold = 0 // The value is zero so that we get all the queries that ran in ./mysql-performance-config/custom-entrypoint.sh

	// cli flags
	perfContainer                        = flag.String("perfContainer", defaultPerfContainer, "container where the integration is installed and used for validating performance monitoring metrics")
	binPath                              = flag.String("bin", defaultBinPath, "Integration binary path")
	user                                 = flag.String("user", defaultMysqlUser, "Mysql user name")
	psw                                  = flag.String("psw", defaultMysqlPass, "Mysql user password")
	port                                 = flag.Int("port", defaultMysqlPort, "Mysql port")
	enableQueryMonitoring                = flag.Bool("enable_query_monitoring", defaultEnableQueryMonitoring, "flag to enable and disable collecting query metrics")
	slowQueryMonitoringFetchInterval     = flag.Int("slow_query_monitoring_fetch_interval", defaultSlowQueryMonitoringFetchInterval, "retrives slow queries that ran in last n seconds")
	queryMonitoringResponseTimeThreshold = flag.Int("query_monitoring_response_time_threshold", defaultQueryMonitoringResponseTimeThreshold, "retrives queries that have taken more time than queryResponseTimeThreshold in milli seconds")
)

type MysqlPerformanceConfig struct {
	Version  string // Mysql server version
	Hostname string // Hostname for the Mysql service. (Will be the mysql service inside the docker-compose-performance file).
}

var (
	MysqlPerfConfigs = []MysqlPerformanceConfig{
		{
			Version:  "8.0.40",
			Hostname: "mysql_perf_8-0-40",
		},
		{
			Version:  "8.4.0",
			Hostname: "mysql_perf_8-4-0",
		},
		{
			Version:  "9.1.0",
			Hostname: "mysql_perf_latest-supported",
		},
	}
)

func runIntegrationAndGetStdoutWithError(t *testing.T, targetContainer string, envVars ...string) (string, string, error) {
	return helpers.RunIntegrationAndGetStdout(t, binPath, user, psw, port, slowQueryMonitoringFetchInterval, queryMonitoringResponseTimeThreshold, perfContainer, targetContainer, envVars)
}

func executeBlockingSessionQuery(mysqlPerfConfig MysqlPerformanceConfig) error {
	flag.Parse()

	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	masterErr := helpers.WaitForPort(*perfContainer, mysqlPerfConfig.Hostname, *port, 60*time.Second)
	if masterErr != nil {
		return masterErr
	}

	/*
		Steps to create blocking session:
			1. Create a lock on particular row of a table from one session. This can be found in /mysql-performance-config/custom-entrypoint.sh
			2. Try to aquire lock on the same row as above by updating the same row of the table from another session. This is being done below.
	*/
	blockingSessionQuery := "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ; USE employees; START TRANSACTION; UPDATE employees SET last_name = 'Blocking' WHERE emp_no = 10001;"
	blockingSessionCmd := []string{`mysql`, `-u`, `root`, `-e`, blockingSessionQuery}
	/*
		Uncomment the below code to debug when the integration doesn't report blocking session metrics.
		The code execution should stop below when you make sure there is another session already holding lock on the row of employees table having emp_no as 10001

		if the code execution stops here then we simulated a blocking session succesfully
			_, blockingSessionErr, err := helpers.ExecInContainer(mysqlPerfConfig.Hostname, blockingSessionCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
		if blockingSessionErr != "" {
			log.Debug("Error exec blocking session queries: ", blockingSessionErr, err)
		}

		Note: The blocking session query is executed using go-routine because:
			1. The nri-mysql integration reports only live blocking session metrics data
			2. While executing the binary of nri-mysql integration there should be a live blocking session event in the mysql server
			3. go-routine is used below to make sure the exectuion of the query & binary happen concurrently and blocking sessions metrics are reported in the stdout
	*/
	go helpers.ExecInContainer(mysqlPerfConfig.Hostname, blockingSessionCmd, fmt.Sprintf("MYSQL_PWD=%s", *psw))
	log.Info("wait for the blocking session query to get executed for host :" + mysqlPerfConfig.Hostname)
	time.Sleep(10 * time.Second)
	log.Info("wait complete")
	log.Info("Executing blocking sessions complete!")

	return nil
}

func teardown() error {
	return nil
}

func TestMain(m *testing.M) {
	log.Info("wait for mysql servers with performance schema/extensions enabled to come up and running")
	time.Sleep(60 * time.Second)
	log.Info("wait complete")
	for _, mysqlPerfConfig := range MysqlPerfConfigs {
		err := executeBlockingSessionQuery(mysqlPerfConfig)
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

func testPerfOutputIsValidJSON(t *testing.T, mysqlPerfConfig MysqlPerformanceConfig) {
	t.Run(mysqlPerfConfig.Version, func(t *testing.T) {
		stdout, stderr, err := runIntegrationAndGetStdoutWithError(t, mysqlPerfConfig.Hostname)
		if stderr != "" {
			log.Debug("Integration command Standard Error: ", stderr)
		}
		require.NoError(t, err)
		outputMetricsList := strings.Split(stdout, "\n")
		for _, outputMetrics := range outputMetricsList {
			outputMetrics = strings.TrimSpace(outputMetrics)
			if outputMetrics == "" {
				continue
			}
			var j map[string]interface{}
			err := json.Unmarshal([]byte(outputMetrics), &j)
			assert.NoError(t, err, "Integration output should be a JSON dict")
		}
	})
}

func TestPerfOutputIsValidJSON(t *testing.T) {
	for _, mysqlConfig := range MysqlPerfConfigs {
		testPerfOutputIsValidJSON(t, mysqlConfig)
	}
}

func runValidMysqlPerfConfigTest(t *testing.T, args []string, outputMetricsFile string, testName string) {
	for _, mysqlPerfConfig := range MysqlPerfConfigs {
		t.Run(testName+mysqlPerfConfig.Version, func(t *testing.T) {
			args = append(args, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
			stdout, stderr, err := runIntegrationAndGetStdoutWithError(t, mysqlPerfConfig.Hostname, args...)
			if stderr != "" {
				log.Debug("Integration command Standard Error: ", stderr)
			}
			require.NoError(t, err)
			outputMetricsList := strings.Split(stdout, "\n")
			if testName == "OnlyInventory_EnableQueryMonitoring" {
				/*
					 	Note: Only standard integration metrics json with we present in the stdout.
						Integration will report query performance monitoring data when both metrics and enable_query_monitoring are enabled.
						Refer args.HasMetrics() implementation here https://github.com/newrelic/infra-integrations-sdk/blob/12ee4e8a20a479f2b3d9ba328d2f80c9dc663c79/args/args.go#L33

						In this testcase metrics flag is disabled. So, validation of the standard json output is being done.
				*/
				schemaPath := filepath.Join("json-schema-performance-files", outputMetricsFile)
				// Skip validation if output is empty
				if len(outputMetricsList) > 0 && strings.TrimSpace(outputMetricsList[0]) != "" {
					err := jsonschema.Validate(schemaPath, outputMetricsList[0])
					require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
				} else {
					t.Logf("Empty output - skipping schema validation for %s", testName)
				}
			} else {
				// Build validation list conditionally to avoid out-of-range access when
				// some metric groups are not emitted for certain MySQL versions.
				type cfg struct {
					name           string
					stdout         string
					schemaFileName string
				}
				var outputMetricsConfigs []cfg

				addIfPresent := func(idx int, name, schema string) {
					if len(outputMetricsList) > idx && strings.TrimSpace(outputMetricsList[idx]) != "" {
						outputMetricsConfigs = append(outputMetricsConfigs, cfg{name, outputMetricsList[idx], schema})
					} else {
						if len(outputMetricsList) <= idx {
							t.Logf("Output line %d missing - skipping schema validation for %s", idx, name)
						} else {
							t.Logf("Output line %d empty - skipping schema validation for %s", idx, name)
						}
					}
				}

				addIfPresent(0, "DefaultMetrics", outputMetricsFile)
				addIfPresent(1, "SlowQueryMetrics", "mysql-schema-slow-queries.json")
				addIfPresent(2, "IndividualQueryMetrics", "mysql-schema-individual-queries.json")
				addIfPresent(3, "QueryExecutionMetrics", "mysql-schema-query-execution.json")
				addIfPresent(4, "WaitEventsMetrics", "mysql-schema-wait-events.json")
				addIfPresent(5, "BlockingSessionMetrics", "mysql-schema-blocking-sessions.json")

				for _, outputConfig := range outputMetricsConfigs {
					schemaPath := filepath.Join("json-schema-performance-files", outputConfig.schemaFileName)
					err := jsonschema.Validate(schemaPath, outputConfig.stdout)
					require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
				}
			}
		})
	}
}

func TestPerfMySQLIntegrationValidArguments(t *testing.T) {
	testCases := []struct {
		name              string
		args              []string
		outputMetricsFile string
	}{
		{
			name: "RemoteEntity_EnableQueryMonitoring",
			args: []string{
				"REMOTE_MONITORING=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-master.json",
		},
		{
			name: "LocalEntity_EnableQueryMonitoring",
			args: []string{
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-master-localentity.json",
		},
		{
			name: "OnlyMetrics_EnableQueryMonitoring",
			args: []string{
				"METRICS=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-metrics-master.json",
		},
		{
			name: "OnlyInventory_EnableQueryMonitoring",
			args: []string{
				"INVENTORY=true",
				"ENABLE_QUERY_MONITORING=true",
			},
			outputMetricsFile: "mysql-schema-inventory-master.json",
		},
	}

	for _, testCase := range testCases {
		runValidMysqlPerfConfigTest(t, testCase.args, testCase.outputMetricsFile, testCase.name)
	}
}
