package database

type Driver string

const (
	DriverPostgres  Driver = "postgres"
	DriverCockroach Driver = "cockroach"
	DriverMySQL     Driver = "mysql"
	DriverMSSQL     Driver = "mssql"
)
