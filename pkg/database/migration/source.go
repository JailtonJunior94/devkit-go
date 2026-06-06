package migration

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type Source interface {
	sourceMarker()
}

type FSPath string

func (FSPath) sourceMarker() {}

type EmbedFS struct {
	FS   fs.FS
	Root string
}

func (EmbedFS) sourceMarker() {}

func resolveSource(src Source) (drv source.Driver, urlStr string, err error) {
	switch s := src.(type) {
	case FSPath:
		return nil, "file://" + string(s), nil
	case EmbedFS:
		d, err := iofs.New(s.FS, s.Root)
		if err != nil {
			return nil, "", fmt.Errorf("migration: iofs source: %w", err)
		}
		return d, "", nil
	default:
		return nil, "", fmt.Errorf("migration: unsupported source type %T", src)
	}
}
