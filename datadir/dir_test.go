package datadir

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRootDir(t *testing.T) {
	path := "/tmp/waypoint"
	rootDir, err := newRootDir(path)

	expectedCacheDir := path + "/cache"
	expectedDataDir := path + "/data"

	assert.Nil(t, err)
	assert.Equal(t, rootDir.CacheDir(), expectedCacheDir)
	assert.Equal(t, rootDir.DataDir(), expectedDataDir)
}

func TestNewBasicDir(t *testing.T) {
	cacheDirStr := "/tmp/cache"
	dataDirStr := "/tmp/data"

	basicDir := NewBasicDir(cacheDirStr, dataDirStr)

	assert.Equal(t, basicDir.CacheDir(), cacheDirStr)
	assert.Equal(t, basicDir.DataDir(), dataDirStr)
}

func TestNewScopedDir(t *testing.T) {
	cacheDirStr := "/tmp/cache"
	dataDirStr := "/tmp/data"
	path := "/waypoint"
	parent := NewBasicDir(cacheDirStr, dataDirStr)

	scopedDir, err := NewScopedDir(parent, path)

	expectedCacheDir := cacheDirStr + path
	expectedDataDir := dataDirStr + path

	assert.Nil(t, err)
	assert.Equal(t, scopedDir.CacheDir(), expectedCacheDir)
	assert.Equal(t, scopedDir.DataDir(), expectedDataDir)
}
