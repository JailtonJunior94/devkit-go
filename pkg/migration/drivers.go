package migration

// Driver represents the supported database drivers for migrations.
type Driver string

const (
	// DriverPostgres represents PostgreSQL database driver.
	DriverPostgres Driver = "postgres"

	// DriverCockroachDB represents CockroachDB database driver.
	// Note: CockroachDB is wire-compatible with PostgreSQL but has specific
	// considerations for distributed transactions and locking behavior.
	DriverCockroachDB Driver = "cockroachdb"

	// DriverMySQL represents MySQL database driver.
	// Also compatible with MariaDB.
	DriverMySQL Driver = "mysql"
)

// String returns the string representation of the driver.
func (d Driver) String() string {
	return string(d)
}

// IsValid validates if the driver is supported.
func (d Driver) IsValid() bool {
	switch d {
	case DriverPostgres, DriverCockroachDB, DriverMySQL:
		return true
	default:
		return false
	}
}

// ToMigrateDriver converts the driver to the golang-migrate driver name.
func (d Driver) ToMigrateDriver() string {
	switch d {
	case DriverPostgres:
		return "postgres"
	case DriverCockroachDB:
		return "cockroachdb"
	case DriverMySQL:
		return "mysql"
	default:
		return ""
	}
}
