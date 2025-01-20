package utils

import (
	"context"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	constants "github.com/newrelic/nri-mysql/src/query-performance-monitoring/constants"
)

type DataSource interface {
	Close()
	QueryX(string) (*sqlx.Rows, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
}

type Database struct {
	source *sqlx.DB
}

// OpenDB function creates and returns a connection using the sqlx package for advanced query execution
// and mapping capabilities. It offers methods like Queryx, QueryRowx, etc., that facilitate working
// with structs, slices, and named queries, making it well-suited for applications needing sophisticated
// data handling compared to the standard sql package.
func OpenDB(dsn string) (DataSource, error) {
	source, err := sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening DSN: %w", err)
	}

	db := Database{
		source: source,
	}

	return &db, nil
}

func (db *Database) Close() {
	db.source.Close()
}

func (db *Database) QueryX(query string) (*sqlx.Rows, error) {
	rows, err := db.source.Queryx(query)
	return rows, err
}

// QueryxContext method implementation
func (db *Database) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return db.source.QueryxContext(ctx, query, args...)
}

// collectMetrics collects metrics from the performance schema database
func CollectMetrics[T any](db DataSource, preparedQuery string, preparedArgs ...interface{}) ([]T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutDuration)
	defer cancel()

	rows, err := db.QueryxContext(ctx, preparedQuery, preparedArgs...)
	if err != nil {
		return []T{}, err
	}
	defer rows.Close()

	var metrics []T
	for rows.Next() {
		var metric T
		if err := rows.StructScan(&metric); err != nil {
			return []T{}, err
		}
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return []T{}, err
	}

	return metrics, nil
}
