package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDatabaseFlavor(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		expectedFlavor DatabaseFlavor
	}{
		{
			name:         "MySQL 8.0",
			version:      "8.0.23",
			expectedFlavor: DatabaseFlavorMySQL,
		},
		{
			name:         "MySQL 8.4",
			version:      "8.4.0",
			expectedFlavor: DatabaseFlavorMySQL,
		},
		{
			name:         "MySQL 5.7",
			version:      "5.7.31",
			expectedFlavor: DatabaseFlavorMySQL,
		},
		{
			name:         "MariaDB 10.6",
			version:      "10.6.0-MariaDB",
			expectedFlavor: DatabaseFlavorMariaDB,
		},
		{
			name:         "MariaDB 10.5",
			version:      "10.5.12-MariaDB-0ubuntu0.20.04.1",
			expectedFlavor: DatabaseFlavorMariaDB,
		},
		{
			name:         "MariaDB case insensitive",
			version:      "10.6.0-maria",
			expectedFlavor: DatabaseFlavorMariaDB,
		},
		{
			name:         "MariaDB uppercase",
			version:      "10.6.0-MARIADB",
			expectedFlavor: DatabaseFlavorMariaDB,
		},
		{
			name:         "MariaDB mixed case",
			version:      "10.6.0-MaRiADb",
			expectedFlavor: DatabaseFlavorMariaDB,
		},
		{
			name:         "MySQL with maria in path but not DB name",
			version:      "8.0.23-mysql-commercial",
			expectedFlavor: DatabaseFlavorMySQL,
		},
		{
			name:         "Empty version string",
			version:      "",
			expectedFlavor: DatabaseFlavorMySQL,
		},
		{
			name:         "Unknown version string",
			version:      "unknown-database-1.0.0",
			expectedFlavor: DatabaseFlavorMySQL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectDatabaseFlavor(tt.version)
			assert.Equal(t, tt.expectedFlavor, result)
		})
	}
}