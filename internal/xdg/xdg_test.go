package xdg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHome_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "specgraph"), xdg.ConfigHome())
}

func TestConfigHome_Override(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test-config")
	assert.Equal(t, "/tmp/xdg-test-config/specgraph", xdg.ConfigHome())
}

func TestDataHome_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "share", "specgraph"), xdg.DataHome())
}

func TestStateHome_Default(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "state", "specgraph"), xdg.StateHome())
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	assert.Equal(t, "/tmp/xdg-test/specgraph/config.yaml", xdg.ConfigFile())
}
