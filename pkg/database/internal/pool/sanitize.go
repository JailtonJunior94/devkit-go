package pool

import (
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// ConnInfo holds safe connection metadata for use in logs, spans and OTel attributes.
// Never include DSN, password or other credentials in this struct.
type ConnInfo struct {
	Driver   string
	Host     string
	Port     int
	Database string
}

// SafeAttrs returns OTel attributes derived from ConnInfo.
// Guarantees that no credential (password, DSN) appears in the returned fields.
func SafeAttrs(info ConnInfo) []observability.Field {
	return []observability.Field{
		observability.String("db.system", info.Driver),
		observability.String("db.name", info.Database),
		observability.String("server.address", info.Host),
		observability.String("server.port", fmt.Sprintf("%d", info.Port)),
	}
}
