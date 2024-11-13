package query_performance_details

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
)

type dataSource interface {
	close()
	queryX(string) (*sqlx.Rows, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
}

type database struct {
	source *sqlx.DB
}

func openDB(dsn string) (dataSource, error) {
	source, err := sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %v", dsn, err)
	}
	db := database{
		source: source,
	}
	return &db, nil
}

func (db *database) close() {
	db.source.Close()
}

func (db *database) queryX(query string) (*sqlx.Rows, error) {
	rows, err := db.source.Queryx(query)
	fatalIfErr(err)
	return rows, err
}

// QueryxContext method implementation
func (db *database) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return db.source.QueryxContext(ctx, query, args...)
}

func generateDSN(args arguments.ArgumentList) string {
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
		log.Info("Socket parameter is defined, ignoring host and port parameters")
		return fmt.Sprintf("%s:%s@unix(%s)/%s?%s", args.Username, args.Password, args.Socket, args.Database, query.Encode())
	}

	// Convert hostname and port to DSN address format
	mysqlURL := net.JoinHostPort(args.Hostname, strconv.Itoa(args.Port))

	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", args.Username, args.Password, mysqlURL, args.Database, query.Encode())
}
