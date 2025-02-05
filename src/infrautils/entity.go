package infrautils

import (
	"fmt"
	"log"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/v3/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/v3/data/metric"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
)

// CreateNodeEntity creates a new integration entity for a MySQL node.
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

// MetricSet creates a new metric set with the given attributes.
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

func FatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
