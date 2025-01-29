package dbutils

import (
	"flag"
	"os"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	arguments "github.com/newrelic/nri-mysql/src/args"
	infrautils "github.com/newrelic/nri-mysql/src/infrautils"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"

	"github.com/stretchr/testify/assert"
)

const integrationVersion = "0.0.0"

var args arguments.ArgumentList

func TestGenerateDSNPriorizesCliOverEnvArgs(t *testing.T) {
	os.Setenv("USERNAME", "dbuser")
	os.Setenv("HOSTNAME", "foo")

	os.Args = []string{
		"cmd",
		"-hostname=bar",
		"-port=1234",
		"-password=dbpwd",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(bar:1234)/?", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsOldPasswords(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-old_passwords",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?allowOldPasswords=true", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsEnableTLS(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-enable_tls",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?tls=true", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsInsecureSkipVerify(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-insecure_skip_verify",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?tls=skip-verify", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSupportsExtraConnectionURLArgs(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-extra_connection_url_args=readTimeout=1s&timeout=5s&tls=skip-verify",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@tcp(dbhost:1234)/?readTimeout=1s&timeout=5s&tls=skip-verify", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}

func TestGenerateDSNSocketDiscardPort(t *testing.T) {
	os.Args = []string{
		"cmd",
		"-hostname=dbhost",
		"-username=dbuser",
		"-password=dbpwd",
		"-port=1234",
		"-socket=/path/to/socket/file",
	}
	_, err := integration.New(constants.IntegrationName, integrationVersion, integration.Args(&args))
	infrautils.FatalIfErr(err)

	assert.Equal(t, "dbuser:dbpwd@unix(/path/to/socket/file)/?", GenerateDSN(args, ""))

	flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)
}
