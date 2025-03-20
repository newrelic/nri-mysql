package main

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
				assert.Equal(t, test.expected, actual, "For version %s, expected %v, but got %v", test.dbVersion, test.expected, actual)
			}
		})
	}
}

func TestIsDBVersionLessThan8Point4(t *testing.T) {
	tests := []struct {
		dbVersion string
		expected  bool
	}{
		{"5.7.0", true},
		{"8.0.0", true},
		{"8.4.0", false},
		{"9.1.0", false},
		{"07.5", true},
		{"18.5.2", false},
		{"invalid_major_version.2.1", true},
		{"10.invalid_minor_version.1", true},
	}

	for _, test := range tests {
		t.Run(test.dbVersion, func(t *testing.T) {
			actual := isDBVersionLessThan8Point4(test.dbVersion)
			assert.Equal(t, test.expected, actual)
			if actual != test.expected {
				assert.Equal(t, test.expected, actual, "For version %s, expected %v, but got %v", test.dbVersion, test.expected, actual)
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
				assert.Equal(t, test.expected, actual, "For version %s, expected %v, but got %v", test.dbVersion, test.expected, actual)
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
			actual, err := extractSanitizedDBVersion(test.version)
			if (err == nil) && (!test.shouldFail) {
				assert.Equal(t, test.expected, actual)
			} else if actual != test.expected {
				assert.Equal(t, test.expected, actual, "extractSanitizedVersion(%q) = %s, want %s", test.version, actual, test.expected)
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

func TestIsMariaDBServer(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"8.4.3-standard", false},
		{"8.0.40-0ubuntu0.22.04.1", false},
		{"", false},
		{"invalid", false},
		{"11.3.2-MariaDB-log", true},
		{"5.6.7-MARIA-DB", true},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			actual := isMariaDBServer(test.version)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGetRawDBVersion(t *testing.T) {
	tests := []struct {
		name          string
		mockRows      *sqlmock.Rows
		expected      string
		shouldFail    bool
		expectedError error
	}{
		{
			name:          "Successful dbVersion query",
			mockRows:      sqlmock.NewRows([]string{"version"}).AddRow("8.0.40-0ubuntu0.22.04.1"),
			expected:      "8.0.40-0ubuntu0.22.04.1",
			shouldFail:    false,
			expectedError: nil,
		},
		{
			name:          "Error exec dbVersion query",
			mockRows:      nil,
			expected:      "",
			shouldFail:    true,
			expectedError: fmt.Errorf("error fetching dbVersion: %w", assert.AnError),
		},
		{
			name:          "Version not found in result",
			mockRows:      sqlmock.NewRows([]string{""}).AddRow(nil),
			expected:      "",
			shouldFail:    true,
			expectedError: fmt.Errorf("%w", errVersionNotFound),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err)
			defer db.Close()

			assert.NoError(t, err)
			database := &database{source: db}

			if test.mockRows == nil && test.shouldFail {
				mock.ExpectQuery(dbVersionQuery).WillReturnError(assert.AnError)
			} else {
				mock.ExpectQuery(dbVersionQuery).WillReturnRows(test.mockRows)
			}

			actual, err := getRawDBVersion(database)
			if test.shouldFail {
				assert.Error(t, test.expectedError, err)
			} else {
				assert.Equal(t, test.expected, actual)
			}
		})
	}
}

func TestCheckDBServerAndGetDBVersion(t *testing.T) {
	tests := []struct {
		name     string
		mockRows *sqlmock.Rows
		expected string
	}{
		{
			name:     "Successful dbVersion query",
			mockRows: sqlmock.NewRows([]string{"version"}).AddRow("8.0.40-0ubuntu0.22.04.1"),
			expected: "8.0.40",
		},
		{
			name:     "dbVersion query output with mariadb",
			mockRows: sqlmock.NewRows([]string{"version"}).AddRow("11.3.2-MariaDB-log"),
			expected: "5.7.0",
		},
		{
			name:     "Error exec dbVersion query",
			mockRows: nil,
			expected: "5.7.0",
		},
		{
			name:     "Version not found in result",
			mockRows: sqlmock.NewRows([]string{""}).AddRow(nil),
			expected: "5.7.0",
		},
		{
			name:     "Invalid dbVersion output",
			mockRows: sqlmock.NewRows([]string{"version"}).AddRow("invalid"),
			expected: "5.7.0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err)
			defer db.Close()

			assert.NoError(t, err)
			database := &database{source: db}
			if test.mockRows != nil {
				mock.ExpectQuery(dbVersionQuery).WillReturnRows(test.mockRows)
			} else {
				mock.ExpectQuery(dbVersionQuery).WillReturnError(assert.AnError)
			}

			actual := checkDBServerAndGetDBVersion(database)
			assert.Equal(t, test.expected, actual)
		})
	}
}
