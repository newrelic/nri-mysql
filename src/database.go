package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/newrelic/infra-integrations-sdk/log"
)

type dataSource interface {
	close()
	query(string) (map[string]interface{}, error)
	queryRows(string) ([]map[string]interface{}, error)
}

type database struct {
	source *sql.DB
}

func openDB(dsn string) (dataSource, error) {
	source, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db := database{
		source: source,
	}
	return &db, nil
}

func (db *database) close() {
	db.source.Close()
}

// query executes provided as an argument query and parses the output to the map structure.
// It is only possible to parse two types of query:
// 1. output of the query consists of two columns. Names of the columns are ignored. Values from the first
// column are used as keys, and from the second as corresponding values of the map. Number of rows can be greater than 1;
// 2. output of the query consists of multiple columns, but only single row.
// In this case, each column name is a key, and corresponding value is a map value.
func (db *database) query(query string) (map[string]interface{}, error) {
	rows, err := db.source.Query(query)
	if err != nil {
		return nil, err
	}

	rawData := make(map[string]interface{})

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rowIndex := 0
	for rows.Next() {
		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, err
		}

		if len(values) == 2 {
			rawData[string(values[0])] = asValue(string(values[1]))
		} else {
			if rowIndex != 0 {
				log.Debug("Cannot process query: %s, for query output with more than 2 columns only single row expected", query)
				break
			}

			for i, value := range values {
				rawData[columns[i]] = asValue(string(value))
			}
			rowIndex++
		}
	}

	return rawData, nil
}

func (db *database) queryRows(query string) ([]map[string]interface{}, error) {
	var rawRows []map[string]interface{}

	rows, err := db.source.Query(query)
	if err != nil {
		return nil, err
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		rawData := make(map[string]interface{})

		values := make([]sql.RawBytes, len(columns))
		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, err
		}

		for i := range values {
			rawData[columns[i]] = asValue(string(values[i]))
		}
		rawRows = append(rawRows, rawData)
	}

	return rawRows, nil
}
