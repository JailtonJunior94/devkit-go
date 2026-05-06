package migration

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Source é uma interface selada para fontes de arquivos de migração.
// Apenas FSPath e EmbedFS a satisfazem; chamadores não podem adicionar novas implementações.
type Source interface {
	sourceMarker()
}

// FSPath é um caminho de diretório do sistema de arquivos contendo arquivos SQL de migração.
// o golang-migrate o resolve via o esquema de URL file://.
type FSPath string

func (FSPath) sourceMarker() {}

// EmbedFS envolve um fs.FS com o subdiretório raiz contendo os arquivos de migração.
// Aceita embed.FS em produção; aceita os.DirFS em testes (ambos satisfazem fs.FS).
type EmbedFS struct {
	FS   fs.FS
	Root string
}

func (EmbedFS) sourceMarker() {}

// resolveSource converte um Source em um source.Driver e/ou URL do golang-migrate.
// Para FSPath: retorna nenhum driver e uma URL file:// (migrate.New a usa diretamente).
// Para EmbedFS: retorna um driver iofs e uma URL vazia (use NewWithSourceInstance).
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
