package utils

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	arguments "github.com/newrelic/nri-mysql/src/args"
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
	fatalIfErr(err)
	return rows, err
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// QueryxContext method implementation
func (db *Database) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return db.source.QueryxContext(ctx, query, args...)
}

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
