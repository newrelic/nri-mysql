package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDBVersionLessThan8(t *testing.T) {
	tests := []struct {
		dbVersion string
		expected  bool
	}{
		{"5.6.0", true},
		{"8.0.0", false},
		{"9.1.0", false},
		{"07.5", true},
		{"18.5.2", false},
	}

	for _, test := range tests {
		t.Run(test.dbVersion, func(t *testing.T) {
			actual := isDBVersionLessThan8(test.dbVersion)
			assert.Equal(t, test.expected, actual)
			if actual != test.expected {
				t.Errorf("For version %s, expected %v, but got %v", test.dbVersion, test.expected, actual)
			}
		})
	}
}

func TestGetReplicaQuery(t *testing.T) {
	tests := []struct {
		dbVersion string
		expected  string
	}{
		{"5.6.0", replicaQueryBelowVersion8Point4},
		{"8.0.40", replicaQueryBelowVersion8Point4},
		{"8.4.0", replicaQueryForVersion8Point4AndAbove},
		{"9.1.0", replicaQueryForVersion8Point4AndAbove},
		{"07.5", replicaQueryBelowVersion8Point4},
		{"18.5.2", replicaQueryForVersion8Point4AndAbove},
	}

	for _, test := range tests {
		t.Run(test.dbVersion, func(t *testing.T) {
			actual := getReplicaQuery(test.dbVersion)
			assert.Equal(t, test.expected, actual)
			if actual != test.expected {
				t.Errorf("For version %s, expected %v, but got %v", test.dbVersion, test.expected, actual)
			}
		})
	}
}

func TestExtractSanitizedVersion(t *testing.T) {
	tests := []struct {
		version    string
		expected   string
		shouldFail bool
	}{
		{"8.4.3-standard", "8.4.3", false},
		{"8.0.40-0ubuntu0.22.04.1", "8.0.40", false},
		{"", "", true},
		{"invalid", "", true},
		{"10.5", "10.5.0", false},
		{"5.4.3.2", "5.4.3", false},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			actual, err := extractSanitizedVersion(test.version)
			if (err == nil) && (!test.shouldFail) {
				assert.Equal(t, test.expected, actual)
			} else if actual != test.expected {
				t.Errorf("extractSanitizedVersion(%q) = %s, want %s", test.version, actual, test.expected)
			}
		})
	}
}

func TestGetRawDataWithoutDBVersion(t *testing.T) {
	database := testdb{
		inventory: map[string]interface{}{
			"key_cache_block_size": 10,
			"key_buffer_size":      10,
			"version_comment":      "mysql",
			"version":              "5.7.0",
		},
		metrics: map[string]interface{}{},
		replica: map[string]interface{}{},
		version: map[string]interface{}{},
	}
	inventory, metrics, dbVersion, err := getRawData(database)
	assert.Equal(t, "5.7.0", dbVersion)
	if err != nil {
		t.Error()
	}
	if metrics == nil {
		t.Error()
	}
	if inventory == nil {
		t.Error()
	}
	if dbVersion == "" {
		t.Error()
	}
}
