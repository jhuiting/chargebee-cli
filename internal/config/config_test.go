package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jhuiting/chargebee-cli/internal/config"
)

func TestNewConfig(t *testing.T) {
	cfg := config.NewConfig()

	if cfg.DefaultProfile != "default" {
		t.Errorf("expected default_profile to be %q, got %q", "default", cfg.DefaultProfile)
	}
	if cfg.Profiles == nil {
		t.Fatal("expected profiles map to be initialized")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(cfg.Profiles))
	}
}

func TestSetAndRemoveProfile(t *testing.T) {
	cfg := config.NewConfig()

	cfg.SetProfile("test", &config.Profile{
		Site:   "mysite-test",
		APIKey: "test_key_123",
		Region: "us",
	})

	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(cfg.Profiles))
	}

	p := cfg.Profiles["test"]
	if p.Site != "mysite-test" {
		t.Errorf("expected site %q, got %q", "mysite-test", p.Site)
	}
	if p.APIKey != "test_key_123" {
		t.Errorf("expected api_key %q, got %q", "test_key_123", p.APIKey)
	}

	cfg.RemoveProfile("test")
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected 0 profiles after removal, got %d", len(cfg.Profiles))
	}
}

func TestActiveProfile(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SetProfile("default", &config.Profile{Site: "site1", APIKey: "key1"})
	cfg.SetProfile("staging", &config.Profile{Site: "site2", APIKey: "key2"})

	p, err := cfg.ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Site != "site1" {
		t.Errorf("expected site %q, got %q", "site1", p.Site)
	}

	t.Setenv("CB_PROFILE", "staging")
	p, err = cfg.ActiveProfile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Site != "site2" {
		t.Errorf("expected site %q from CB_PROFILE, got %q", "site2", p.Site)
	}
}

func TestActiveProfileNotFound(t *testing.T) {
	cfg := config.NewConfig()

	_, err := cfg.ActiveProfile()
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CB_CONFIG_DIR", tmpDir)

	cfg := config.NewConfig()
	cfg.SetProfile("default", &config.Profile{
		Site:   "roundtrip-site",
		APIKey: "roundtrip-key",
		Region: "eu",
	})

	if err := config.Save(cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	configFile := filepath.Join(tmpDir, "config.toml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	p, ok := loaded.Profiles["default"]
	if !ok {
		t.Fatal("expected default profile in loaded config")
	}
	if p.Site != "roundtrip-site" {
		t.Errorf("expected site %q, got %q", "roundtrip-site", p.Site)
	}
	if p.APIKey != "roundtrip-key" {
		t.Errorf("expected api_key %q, got %q", "roundtrip-key", p.APIKey)
	}
	if p.Region != "eu" {
		t.Errorf("expected region %q, got %q", "eu", p.Region)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	t.Setenv("CB_CONFIG_DIR", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty profiles for non-existent file, got %d", len(cfg.Profiles))
	}
}
