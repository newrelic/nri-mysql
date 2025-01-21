package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
)

// Dynamic error
var (
	ErrEssentialConsumerNotEnabled   = errors.New("essential consumer is not enabled")
	ErrEssentialInstrumentNotEnabled = errors.New("essential instrument is not fully enabled")
	ErrMySQLVersion                  = errors.New("failed to determine MySQL version")
	ErrModelIsNotValid               = errors.New("model is not a valid struct")
)

func GenerateDSN(args arguments.ArgumentList, database string) string {
	query := url.Values{}
	if args.OldPasswords {
		query.Add("allowOldPasswords", "true")
	}
	if args.EnableTLS {
		query.Add("tls", "true")
	}
	if args.InsecureSkipVerify {
		query.Add("tls", "skip-verify")
	}
	extraArgsMap, err := url.ParseQuery(args.ExtraConnectionURLArgs)
	if err == nil {
		for k, v := range extraArgsMap {
			query.Add(k, v[0])
		}
	} else {
		log.Warn("Could not successfully parse ExtraConnectionURLArgs.", err.Error())
	}
	if args.Socket != "" {
		log.Debug("Socket parameter is defined, ignoring host and port parameters")
		return fmt.Sprintf("%s:%s@unix(%s)/%s?%s", args.Username, args.Password, args.Socket, determineDatabase(args, database), query.Encode())
	}

	// Convert hostname and port to DSN address format
	mysqlURL := net.JoinHostPort(args.Hostname, strconv.Itoa(args.Port))

	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", args.Username, args.Password, mysqlURL, determineDatabase(args, database), query.Encode())
}

// determineDatabase determines which database name to use for the DSN.
func determineDatabase(args arguments.ArgumentList, database string) string {
	if database != "" {
		return database
	}
	return args.Database
}

func CreateNodeEntity(
	i *integration.Integration,
	remoteMonitoring bool,
	hostname string,
	port int,
) (*integration.Entity, error) {
	if remoteMonitoring {
		return i.Entity(fmt.Sprint(hostname, ":", port), constants.NodeEntityType)
	}
	return i.LocalEntity(), nil
}

func CreateMetricSet(e *integration.Entity, sampleName string, args arguments.ArgumentList) *metric.Set {
	return MetricSet(
		e,
		sampleName,
		args.Hostname,
		args.Port,
		args.RemoteMonitoring,
	)
}

func MetricSet(e *integration.Entity, eventType, hostname string, port int, remoteMonitoring bool) *metric.Set {
	if remoteMonitoring {
		return e.NewMetricSet(
			eventType,
			attribute.Attr("hostname", hostname),
			attribute.Attr("port", strconv.Itoa(port)),
		)
	}

	return e.NewMetricSet(
		eventType,
		attribute.Attr("port", strconv.Itoa(port)),
	)
}

func getUniqueExcludedDatabases(excludedDBList []string) []string {
	// Create a map to store unique databases
	uniqueDatabases := make(map[string]struct{})

	// Populate the map with default excluded databases
	for _, dbName := range constants.DefaultExcludedDatabases {
		uniqueDatabases[dbName] = struct{}{}
	}

	// Populate the map with values from excludedDBList
	for _, dbName := range excludedDBList {
		trimmedDBName := strings.TrimSpace(dbName)
		if trimmedDBName != "" {
			uniqueDatabases[trimmedDBName] = struct{}{}
		}
	}

	// Convert the map keys back into a slice
	result := make([]string, 0, len(uniqueDatabases))
	for dbName := range uniqueDatabases {
		result = append(result, dbName)
	}

	return result
}

// GetExcludedDatabases parses the excluded databases list from a JSON string and returns a list of unique excluded databases.
func GetExcludedDatabases(excludedDatabasesList string) []string {
	// Parse the excluded databases list from JSON string
	var excludedDatabasesSlice []string
	if err := json.Unmarshal([]byte(excludedDatabasesList), &excludedDatabasesSlice); err != nil {
		log.Warn("Error parsing excluded databases list: %v", err)
	}

	// Get unique excluded databases
	excludedDatabases := getUniqueExcludedDatabases(excludedDatabasesSlice)

	return excludedDatabases
}

// Helper function to convert a slice of strings to a slice of interfaces
func ConvertToInterfaceSlice(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

// SetMetric sets a metric in the given metric set.
func SetMetric(metricSet *metric.Set, name string, value interface{}, sourceType string) {
	switch sourceType {
	case "gauge":
		err := metricSet.SetMetric(name, value, metric.GAUGE)
		if err != nil {
			log.Warn("Error setting gauge metric: %v", err)
		}
	case "attribute":
		err := metricSet.SetMetric(name, value, metric.ATTRIBUTE)
		if err != nil {
			log.Warn("Error setting attribute metric: %v", err)
		}
	default:
		err := metricSet.SetMetric(name, value, metric.GAUGE)
		if err != nil {
			log.Warn("Error setting default gauge metric: %v", err)
		}
	}
}

// IngestMetric ingests a list of metrics into the integration.
func IngestMetric(metricList []interface{}, eventName string, i *integration.Integration, args arguments.ArgumentList) error {
	instanceEntity, err := CreateNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
	if err != nil {
		log.Error("Error creating entity: %v", err)
		return err
	}

	metricCount := 0
	for _, model := range metricList {
		if model == nil {
			continue
		}
		metricCount++
		err := processModel(model, instanceEntity, eventName, args)
		if err != nil {
			log.Error("Error processing model: %v", err)
			return err
		}
		if metricCount > constants.MetricSetLimit {
			metricCount = 0
			if err = publishMetrics(i); err != nil {
				return err
			}
			instanceEntity, err = CreateNodeEntity(i, args.RemoteMonitoring, args.Hostname, args.Port)
			if err != nil {
				log.Error("Error creating entity: %v", err)
				return err
			}
		}
	}

	if metricCount > 0 {
		if err := publishMetrics(i); err != nil {
			return err
		}
	}

	return nil
}

func processModel(model interface{}, instanceEntity *integration.Entity, eventName string, args arguments.ArgumentList) error {
	metricSet := CreateMetricSet(instanceEntity, eventName, args)

	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Ptr {
		modelValue = modelValue.Elem()
	}
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Struct {
		return ErrModelIsNotValid
	}

	modelType := reflect.TypeOf(model)
	for i := 0; i < modelValue.NumField(); i++ {
		field := modelValue.Field(i)
		fieldType := modelType.Field(i)
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if field.Kind() == reflect.Ptr && !field.IsNil() {
			SetMetric(metricSet, metricName, field.Elem().Interface(), sourceType)
		} else if field.Kind() != reflect.Ptr {
			SetMetric(metricSet, metricName, field.Interface(), sourceType)
		}
	}
	return nil
}

func publishMetrics(i *integration.Integration) error {
	err := i.Publish()
	if err != nil {
		log.Error("Error publishing metrics: %v", err)
		return err
	}
	return nil
}

func FatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
