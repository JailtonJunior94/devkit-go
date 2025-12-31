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
)

// String returns the string representation of the driver.
func (d Driver) String() string {
	return string(d)
}

// IsValid validates if the driver is supported.
func (d Driver) IsValid() bool {
	switch d {
	case DriverPostgres, DriverCockroachDB:
		return true
	default:
		return false
	}
}

// ToMigrateDriver converts the driver to the golang-migrate driver name.
// Both postgres and cockroachdb use the same "postgres" driver in golang-migrate.
func (d Driver) ToMigrateDriver() string {
	switch d {
	case DriverPostgres, DriverCockroachDB:
		return "postgres"
	default:
		return ""
	}
}
