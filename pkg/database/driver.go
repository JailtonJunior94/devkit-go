package database

// Driver identifica o backend do banco de dados.
type Driver string

const (
	DriverPostgres  Driver = "postgres"
	DriverCockroach Driver = "cockroach"
	DriverMySQL     Driver = "mysql"
	DriverMSSQL     Driver = "mssql"
)
