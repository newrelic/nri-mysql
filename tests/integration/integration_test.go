
package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	defaultBinPath      = "/nr-mysql"
	defaultMysqlUser    = "root"
	defaultMysqlPass    = "DBpwd1234!"
	defaultMysqlHost    = "mysql"
	defaultMysqlTLSHost = "mysql-tls"
	defaultMysqlPort    = 3306
	defaultMysqlDB      = "database"

	// cli flags
	container = flag.String("container", defaultContainer, "container where the integration is installed")
	binPath   = flag.String("bin", defaultBinPath, "Integration binary path")
	user      = flag.String("user", defaultMysqlUser, "Mysql user name")
	psw       = flag.String("psw", defaultMysqlPass, "Mysql user password")
	host      = flag.String("host", defaultMysqlHost, "Mysql host ip address")
	tlsHost   = flag.String("tlsHost", defaultMysqlTLSHost, "Mysql TLS host ip address")
	port      = flag.Int("port", defaultMysqlPort, "Mysql port")
	database  = flag.String("database", defaultMysqlDB, "Mysql database")
)

// Returns the standard output, or fails testing if the command returned an error
func runIntegration(t *testing.T, envVars ...string) string {
	t.Helper()

	command := make([]string, 0)
	command = append(command, *binPath)
	if user != nil {
		command = append(command, "--username", *user)
	}
	if psw != nil {
		command = append(command, "--password", *psw)
	}
	if host != nil {
		command = append(command, "--hostname", *host)
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

// Returns the standard output, or error if the connection failed
func runTLSIntegration(t *testing.T, tls, pubKey string) (string, error) {
	t.Helper()

	command := make([]string, 0)
	command = append(command, *binPath)
	if user != nil {
		command = append(command, "--username", *user)
	}
	if psw != nil {
		command = append(command, "--password", *psw)
	}
	if tlsHost != nil {
		command = append(command, "--hostname", *tlsHost)
	}
	if port != nil {
		command = append(command, "--port", strconv.Itoa(*port))
	}
	if database != nil {
		command = append(command, "--database", *database)
	}
	if pubKey != "" {
		command = append(command, "--server_pub_key", pubKey)
	}
	command = append(command, "--tls", tls)

	stdout, stderr, err := helpers.ExecInContainer(*container, command)
	if stderr != "" {
		log.Debug("Integration command Standard Error: ", stderr)
	}

	return stdout, err
}

func setup() error {
	flag.Parse()

	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	return helpers.WaitForPort(*container, *host, *port, 30*time.Second)
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
	stdout := runIntegration(t)

	var j map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestTLSConnection_NoCertificates(t *testing.T) {
	_, err := runTLSIntegration(t, "true", "")
	assert.Error(t, err, "Running a TLS host without TLS configuration should fail")
}

func TestTLSConnection_ProvideCertificates(t *testing.T) {
	stdout, err := runTLSIntegration(t, "true", "/shared/public_key.pem")
	assert.NoError(t, err, "The TLS connection should have worked")

	var j map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestTLSConnection_SkipVerify(t *testing.T) {
	stdout, err := runTLSIntegration(t, "skip-verify", "")
	assert.NoError(t, err, "The TLS connection should have worked")

	var j map[string]interface{}
	err = json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestMySQLIntegrationValidArguments_RemoteEntity(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName), "REMOTE_MONITORING=true")
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationValidArguments_LocalEntity(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-master-localentity.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationOnlyMetrics(t *testing.T) {

	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, "METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-metrics-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlyInventory(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, "INTEGRATION=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))
	schemaPath := filepath.Join("json-schema-files", "mysql-schema-inventory-master.json")
	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")

}
