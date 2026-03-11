package cmd_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jhuiting/chargebee-cli/internal/cmd"
	"github.com/jhuiting/chargebee-cli/internal/config"
)

func TestSwitch_SetsDefaultProfile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("site-a", &config.Profile{Site: "site-a", APIKey: "key-a"})
	cfg.SetProfile("site-b", &config.Profile{Site: "site-b", APIKey: "key-b"})
	cfg.DefaultProfile = "site-a"
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch", "site-b"})
	err := root.Execute()
	require.NoError(t, err)

	reloaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "site-b", reloaded.DefaultProfile)
}

func TestSwitch_NonExistentProfile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("site-a", &config.Profile{Site: "site-a", APIKey: "key-a"})
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch", "nonexistent"})
	err := root.Execute()
	assert.ErrorContains(t, err, "does not exist")
}

func TestSwitch_ListProfiles(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("site-a", &config.Profile{Site: "site-a", APIKey: "key-a"})
	cfg.SetProfile("site-b", &config.Profile{Site: "site-b", APIKey: "key-b"})
	cfg.DefaultProfile = "site-a"
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"switch"})
	err := root.Execute()
	require.NoError(t, err)
}

func TestSwitch_NoProfiles(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch"})
	err := root.Execute()
	require.NoError(t, err)
}

func TestSwitch_PrefixMatch(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("staging-site", &config.Profile{Site: "staging-site", APIKey: "key-a"})
	cfg.SetProfile("production-site", &config.Profile{Site: "production-site", APIKey: "key-b"})
	cfg.DefaultProfile = "production-site"
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch", "stag"})
	err := root.Execute()
	require.NoError(t, err)

	reloaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "staging-site", reloaded.DefaultProfile)
}

func TestSwitch_AmbiguousPrefix(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("site-alpha", &config.Profile{Site: "site-alpha", APIKey: "key-a"})
	cfg.SetProfile("site-beta", &config.Profile{Site: "site-beta", APIKey: "key-b"})
	cfg.DefaultProfile = "site-alpha"
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch", "site"})
	err := root.Execute()
	assert.ErrorContains(t, err, "ambiguous")
	assert.ErrorContains(t, err, "site-alpha")
	assert.ErrorContains(t, err, "site-beta")
}

func TestSwitch_ExactMatchPreferredOverPrefix(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", configDir)

	cfg := config.NewConfig()
	cfg.SetProfile("prod", &config.Profile{Site: "prod", APIKey: "key-a"})
	cfg.SetProfile("production", &config.Profile{Site: "production", APIKey: "key-b"})
	cfg.DefaultProfile = "production"
	require.NoError(t, config.Save(cfg))

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"switch", "prod"})
	err := root.Execute()
	require.NoError(t, err)

	reloaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "prod", reloaded.DefaultProfile)
}
