package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (and creates, if missing) the SQLite database at path.
//
// SetMaxOpenConns(1) is intentional: SQLite serializes writes anyway, and
// keeping a single connection avoids "database is locked" errors in a
// simple template. If you outgrow this, switch to a read/write connection
// pool split (many readers via WAL, one writer) instead of raising this.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		path,
	)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		return nil, err
	}
	return conn, nil
}
