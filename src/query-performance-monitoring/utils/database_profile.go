package utils

import "strings"

type DatabaseFlavor string

const (
	DatabaseFlavorMySQL   DatabaseFlavor = "mysql"
	DatabaseFlavorMariaDB DatabaseFlavor = "mariadb"
)

type DatabaseProfile struct {
	Flavor     DatabaseFlavor
	RawVersion string
}

func DetectDatabaseFlavor(version string) DatabaseFlavor {
	if strings.Contains(strings.ToLower(version), "maria") {
		return DatabaseFlavorMariaDB
	}
	return DatabaseFlavorMySQL
}
