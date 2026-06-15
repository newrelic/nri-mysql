package args

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCustomMetricsArguments(t *testing.T) {
	args := ArgumentList{}

	// Test default values
	assert.Equal(t, "", args.CustomMetricsQuery)
	assert.Equal(t, "", args.CustomMetricsConfig)
}