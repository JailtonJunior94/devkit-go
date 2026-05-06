package migration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	migratelib "github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/require"
)

// Verifica que FSPath e EmbedFS satisfazem a interface Source.
func TestFSPath_ImplementsSource(t *testing.T) {
	var _ Source = FSPath("")
}

func TestEmbedFS_ImplementsSource(t *testing.T) {
	var _ Source = EmbedFS{}
}

func TestResolveSource_FSPath_ReturnsFileURL(t *testing.T) {
	drv, urlStr, err := resolveSource(FSPath("/tmp/migrations"))

	require.NoError(t, err)
	require.Nil(t, drv)
	require.Equal(t, "file:///tmp/migrations", urlStr)
}

func TestResolveSource_FSPath_EmptyPath(t *testing.T) {
	drv, urlStr, err := resolveSource(FSPath(""))

	require.NoError(t, err)
	require.Nil(t, drv)
	require.Equal(t, "file://", urlStr)
}

func TestResolveSource_EmbedFS_ValidDir(t *testing.T) {
	tmpDir := t.TempDir()
	writeMigrationFile(t, tmpDir, "000001_create_test.up.sql", "CREATE TABLE test (id INT);")
	writeMigrationFile(t, tmpDir, "000001_create_test.down.sql", "DROP TABLE test;")

	drv, urlStr, err := resolveSource(EmbedFS{FS: os.DirFS(tmpDir), Root: "."})

	require.NoError(t, err)
	require.NotNil(t, drv)
	require.Empty(t, urlStr)

	require.NoError(t, drv.Close())
}

func TestResolveSource_EmbedFS_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	drv, urlStr, err := resolveSource(EmbedFS{FS: os.DirFS(tmpDir), Root: "."})

	require.NoError(t, err)
	require.NotNil(t, drv) // o iofs.New tem sucesso em diretórios vazios
	require.Empty(t, urlStr)

	require.NoError(t, drv.Close())
}

func TestResolveSource_Unsupported_ReturnsError(t *testing.T) {
	_, _, err := resolveSource(unsupportedSource{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported source type")
}

func TestMapError_Nil(t *testing.T) {
	require.NoError(t, mapError(nil))
}

func TestMapError_ErrNoChange_ReturnsSentinel(t *testing.T) {
	err := mapError(migratelib.ErrNoChange)

	require.ErrorIs(t, err, ErrNoChange)
	require.NotEqual(t, ErrNoChange, err, "ErrNoChange é retornado com wrapping; comparações diretas são incorretas")
}

func TestMapError_OtherError_ReturnsMigrationFailed(t *testing.T) {
	original := errors.New("disk full")
	err := mapError(original)

	require.False(t, errors.Is(err, ErrNoChange))
	require.Contains(t, err.Error(), "migration failed")
}

// helpers

func writeMigrationFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

type unsupportedSource struct{}

func (unsupportedSource) sourceMarker() {}
