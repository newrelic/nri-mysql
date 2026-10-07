package args

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCustomMetricsArguments(t *testing.T) {
	args := ArgumentList{}

	// Test default values
	assert.Equal(t, "", args.CustomMetricsQuery)
	assert.Equal(t, "", args.CustomMetricsConfig)
}
