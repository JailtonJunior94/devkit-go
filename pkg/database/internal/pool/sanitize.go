package pool

import (
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type ConnInfo struct {
	Driver   string
	Host     string
	Port     int
	Database string
}

func SafeAttrs(info ConnInfo) []observability.Field {
	return []observability.Field{
		observability.String("db.system", info.Driver),
		observability.String("db.name", info.Database),
		observability.String("server.address", info.Host),
		observability.String("server.port", fmt.Sprintf("%d", info.Port)),
	}
}
