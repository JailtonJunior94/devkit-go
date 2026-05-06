package mssql

import "database/sql"

// applyDefaults sets safe production defaults on db.
// Called in New before applying user-provided overrides from MSSQLConfig.
func applyDefaults(db *sql.DB) {
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLife)
	db.SetConnMaxIdleTime(DefaultConnMaxIdle)
}
