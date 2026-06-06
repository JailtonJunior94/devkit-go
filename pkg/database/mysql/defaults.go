package mysql

import "database/sql"

func applyDefaults(db *sql.DB) {
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLife)
	db.SetConnMaxIdleTime(DefaultConnMaxIdle)
}
