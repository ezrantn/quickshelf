package db

import (
	"database/sql"
	_ "embed"
)

//go:embed schema.sql
var schema string

// Migrate applies the schema. Every statement uses CREATE TABLE/INDEX IF
// NOT EXISTS, so this is safe to call on every boot.
func Migrate(conn *sql.DB) error {
	_, err := conn.Exec(schema)
	return err
}
