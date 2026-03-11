package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Profile holds credentials and settings for a single Chargebee site.
type Profile struct {
	Site   string `toml:"site"`
	APIKey string `toml:"api_key"`
	Region string `toml:"region,omitempty"`
}

// Config is the top-level CLI configuration stored on disk.
type Config struct {
	DefaultProfile string              `toml:"default_profile"`
	Profiles       map[string]*Profile `toml:"profiles"`
}

// NewConfig returns an empty configuration with initialized maps.
func NewConfig() *Config {
	return &Config{
		DefaultProfile: "default",
		Profiles:       make(map[string]*Profile),
	}
}

// ActiveProfile returns the profile currently in use, respecting
// the CB_PROFILE environment variable, then falling back to DefaultProfile.
func (c *Config) ActiveProfile() (*Profile, error) {
	name := os.Getenv("CB_PROFILE")
	if name == "" {
		name = c.DefaultProfile
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return p, nil
}

// SetProfile stores a profile by name.
func (c *Config) SetProfile(name string, p *Profile) {
	c.Profiles[name] = p
}

// RemoveProfile deletes a profile by name.
func (c *Config) RemoveProfile(name string) {
	delete(c.Profiles, name)
}

// Dir returns the configuration directory path (~/.config/chargebee).
func Dir() (string, error) {
	dir := os.Getenv("CB_CONFIG_DIR")
	if dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "chargebee"), nil
}

// FilePath returns the full path to the config file.
func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the configuration from disk. Returns a new empty config
// if the file does not exist.
func Load() (*Config, error) {
	path, err := FilePath()
	if err != nil {
		return nil, err
	}

	cfg := NewConfig()

	data, err := os.ReadFile(path) //nolint:gosec // path is derived from user home directory
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*Profile)
	}
	return cfg, nil
}

// Save writes the configuration to disk, creating the directory if needed.
func Save(cfg *Config) error {
	path, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // path is derived from user home directory
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
