package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindMigrationsDir(t *testing.T) {
	t.Run("env", func(t *testing.T) {
		t.Setenv("MIGRATIONS_PATH", filepath.FromSlash("/tmp/search-migrations"))
		dir, err := findMigrationsDir()
		require.NoError(t, err)
		require.Equal(t, filepath.FromSlash("/tmp/search-migrations"), dir)
	})
	t.Run("walk", func(t *testing.T) {
		t.Setenv("MIGRATIONS_PATH", "")
		cwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(filepath.Join(cwd, "..", "..")))
		t.Cleanup(func() { _ = os.Chdir(cwd) })
		dir, err := findMigrationsDir()
		require.NoError(t, err)
		require.DirExists(t, dir)
		require.FileExists(t, filepath.Join(dir, "000001_create_search.up.sql"))
	})
}
