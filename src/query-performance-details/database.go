package query_performance_details

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type dataSource interface {
	close()
	queryX(string) (*sqlx.Rows, error)
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
