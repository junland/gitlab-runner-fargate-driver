package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/internal/assertions"
)

func TestNewOS(t *testing.T) {
	assert.Implements(t, (*FS)(nil), NewOS())
}

func TestFs_ReadFile(t *testing.T) {
	fs := NewMem()

	data, err := fs.ReadFile("false-file")
	assert.Empty(t, data)
	assertions.ErrorIs(t, err, os.ErrNotExist)
}

func TestFs_WriteFile(t *testing.T) {
	fs := NewMem()

	file := "test-file"
	content := []byte("content")

	err := fs.WriteFile(file, content, 0o600)
	require.NoError(t, err)

	data, err := fs.ReadFile(file)
	assert.Equal(t, content, data)
	assert.NoError(t, err)
}

func TestFs_Exists(t *testing.T) {
	fs := NewMem()

	file := "test-file"
	e, err := fs.Exists(file)
	assert.False(t, e)
	assert.NoError(t, err)

	err = fs.WriteFile(file, nil, 0o600)
	require.NoError(t, err)

	e, err = fs.Exists(file)
	assert.True(t, e)
	assert.NoError(t, err)
}

func TestFs_TempDir(t *testing.T) {
	fs := NewMem()

	dir := "some-dir"
	prefix := "some-prefix"

	path, err := fs.TempDir(dir, prefix)
	assert.Contains(t, path, filepath.Join(dir, prefix))
	assert.NoError(t, err)
}

func TestFs_Remove(t *testing.T) {
	fs := NewMem()

	file := "test-file"
	err := fs.WriteFile(file, nil, 0o600)
	require.NoError(t, err)

	e, err := fs.Exists(file)
	require.True(t, e)
	require.NoError(t, err)

	err = fs.Remove(file)
	assert.NoError(t, err)

	e, err = fs.Exists(file)
	assert.False(t, e)
	assert.NoError(t, err)
}

func TestFs_MkdirAll(t *testing.T) {
	fs := NewMem()

	parent := "some-dir"
	dir := filepath.Join(parent, "nested-dir")

	err := fs.MkdirAll(dir, 0o700)
	require.NoError(t, err)

	for _, path := range []string{parent, dir} {
		e, err := fs.Exists(path)
		assert.True(t, e, path)
		assert.NoError(t, err, path)
	}

	err = fs.MkdirAll(dir, 0o700)
	assert.NoError(t, err, "an existing directory is not an error")
}
