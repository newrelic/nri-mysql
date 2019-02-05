// +build integration

package integration

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitly/go-simplejson"
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
	defaultBinPath   = "/nr-mysql"
	defaultMysqlUser = "root"
	defaultMysqlPass = "DBpwd1234!"
	defaultMysqlHost = "mysql"
	defaultMysqlPort = 3306
	defaultMysqlDB   = "database"

	// cli flags
	container = flag.String("container", defaultContainer, "container where the integration is installed")
	update    = flag.Bool("test.update", false, "update json-schema file")
	binPath   = flag.String("bin", defaultBinPath, "Integration binary path")
	user      = flag.String("user", defaultMysqlUser, "Mysql user name")
	psw       = flag.String("psw", defaultMysqlPass, "Mysql user password")
	host      = flag.String("host", defaultMysqlHost, "Mysql host ip address")
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
		command = append(command, "--port", fmt.Sprint(*port))
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
	stdout := runIntegration(t)

	var j map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &j)
	assert.NoError(t, err, "Integration output should be a JSON dict")
}

func TestMySQLIntegrationValidArguments(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))

	schemaPath := filepath.Join("json-schema-files", "mysql-schema-master.json")
	if *update {
		schema, err := jsonschema.Generate(stdout)
		require.NoError(t, err)

		schemaJSON, err := simplejson.NewJson(schema)
		require.NoError(t, err, "Unmarshaling JSON schema")

		err = helpers.ModifyJSONSchemaGlobal(schemaJSON, iName, 2, "1.2.0")
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaInventoryPresent(schemaJSON)
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaMetricsPresent(schemaJSON, "MysqlSample")
		require.NoError(t, err)

		schema, err = schemaJSON.MarshalJSON()
		require.NoError(t, err, "Marshaling JSON schema")

		err = ioutil.WriteFile(schemaPath, schema, 0644)
		require.NoError(t, err)
	}

	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format")
}

func TestMySQLIntegrationOnlyMetrics(t *testing.T) {

	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, "METRICS=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))

	schemaPath := filepath.Join("json-schema-files", "mysql-schema-metrics-master.json")
	if *update {
		schema, err := jsonschema.Generate(stdout)
		require.NoError(t, err)

		schemaJSON, err := simplejson.NewJson(schema)
		require.NoError(t, err, "Cannot unmarshal JSON schema")

		err = helpers.ModifyJSONSchemaGlobal(schemaJSON, iName, 2, "1.2.0")
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaNoInventory(schemaJSON)
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaMetricsPresent(schemaJSON, "MysqlSample")
		require.NoError(t, err)

		schema, err = schemaJSON.MarshalJSON()
		require.NoError(t, err)

		err = ioutil.WriteFile(schemaPath, schema, 0644)
		require.NoError(t, err)
	}

	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")
}

func TestMySQLIntegrationOnlyInventory(t *testing.T) {
	testName := helpers.GetTestName(t)
	stdout := runIntegration(t, "INTEGRATION=true", fmt.Sprintf("NRIA_CACHE_PATH=/tmp/%v.json", testName))

	schemaPath := filepath.Join("json-schema-files", "mysql-schema-inventory-master.json")
	if *update {
		schema, err := jsonschema.Generate(stdout)
		if err != nil {
			t.Fatal(err)
		}

		schemaJSON, err := simplejson.NewJson(schema)
		require.NoError(t, err, "Cannot unmarshal JSON schema")

		err = helpers.ModifyJSONSchemaGlobal(schemaJSON, iName, 2, "1.2.0")
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaInventoryPresent(schemaJSON)
		require.NoError(t, err)

		err = helpers.ModifyJSONSchemaNoMetrics(schemaJSON)
		require.NoError(t, err)

		schema, err = schemaJSON.MarshalJSON()
		require.NoError(t, err, "Cannot marshal JSON schema")

		err = ioutil.WriteFile(schemaPath, schema, 0644)
		require.NoError(t, err)
	}

	err := jsonschema.Validate(schemaPath, stdout)
	require.NoError(t, err, "The output of MySQL integration doesn't have expected format.")

}
