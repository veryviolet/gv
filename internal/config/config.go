// Package config reads and writes gv's on-disk configuration.
//
// Layout mirrors gh: a general config.yml plus a hosts.yml holding credentials,
// under $GV_CONFIG_DIR, else $XDG_CONFIG_HOME/gv, else ~/.config/gv.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultHost is the GitVerse instance used when nothing else is specified.
const DefaultHost = "gitverse.ru"

// Token source labels, reported by Token so commands can explain where a
// credential came from.
const (
	SourceEnvGV       = "GV_TOKEN"
	SourceEnvGitVerse = "GITVERSE_TOKEN"
	SourceFile        = "hosts.yml"
)

// HostConfig holds the credentials and preferences for a single host.
type HostConfig struct {
	User        string `yaml:"user,omitempty"`
	OAuthToken  string `yaml:"oauth_token,omitempty"`
	GitProtocol string `yaml:"git_protocol,omitempty"`
}

// General holds host-independent preferences.
type General struct {
	GitProtocol string `yaml:"git_protocol,omitempty"`
	Editor      string `yaml:"editor,omitempty"`
	Pager       string `yaml:"pager,omitempty"`
	Prompt      string `yaml:"prompt,omitempty"`
}

type Config struct {
	general General
	hosts   map[string]*HostConfig

	dir string
}

// Dir returns the directory gv keeps its configuration in.
func Dir() string {
	if v := os.Getenv("GV_CONFIG_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "gv")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gv"
	}
	return filepath.Join(home, ".config", "gv")
}

// Load reads configuration from disk. A missing config is not an error: the
// zero configuration is perfectly usable when a token comes from the environment.
func Load() (*Config, error) {
	return loadFrom(Dir())
}

func loadFrom(dir string) (*Config, error) {
	c := &Config{
		hosts: map[string]*HostConfig{},
		dir:   dir,
	}

	if data, err := os.ReadFile(filepath.Join(dir, "config.yml")); err == nil {
		if err := yaml.Unmarshal(data, &c.general); err != nil {
			return nil, fmt.Errorf("parsing config.yml: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if data, err := os.ReadFile(filepath.Join(dir, "hosts.yml")); err == nil {
		if err := yaml.Unmarshal(data, &c.hosts); err != nil {
			return nil, fmt.Errorf("parsing hosts.yml: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	for host, hc := range c.hosts {
		if hc == nil {
			c.hosts[host] = &HostConfig{}
		}
	}
	return c, nil
}

// Token returns the credential for host along with a label describing where it
// came from. Environment variables win over the stored configuration so that
// CI can override a developer's login without touching disk.
func (c *Config) Token(host string) (string, string) {
	if v := os.Getenv("GV_TOKEN"); v != "" {
		return v, SourceEnvGV
	}
	if v := os.Getenv("GITVERSE_TOKEN"); v != "" {
		return v, SourceEnvGitVerse
	}
	if hc, ok := c.hosts[normalizeHost(host)]; ok {
		return hc.OAuthToken, SourceFile
	}
	return "", ""
}

// User returns the stored login for host, or the empty string if unknown.
func (c *Config) User(host string) string {
	if hc, ok := c.hosts[normalizeHost(host)]; ok {
		return hc.User
	}
	return ""
}

// SetToken records a credential for host in memory; call Write to persist it.
func (c *Config) SetToken(host, user, token string) {
	host = normalizeHost(host)
	hc, ok := c.hosts[host]
	if !ok {
		hc = &HostConfig{}
		c.hosts[host] = hc
	}
	hc.User = user
	hc.OAuthToken = token
}

// SetUser records the login for host without touching the token.
func (c *Config) SetUser(host, user string) {
	host = normalizeHost(host)
	if hc, ok := c.hosts[host]; ok {
		hc.User = user
		return
	}
	c.hosts[host] = &HostConfig{User: user}
}

// RemoveHost forgets a host entirely.
func (c *Config) RemoveHost(host string) {
	delete(c.hosts, normalizeHost(host))
}

// Hosts lists the configured hosts in a stable order.
func (c *Config) Hosts() []string {
	out := make([]string, 0, len(c.hosts))
	for h := range c.hosts {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// GitProtocol reports whether to prefer https or ssh URLs for host.
func (c *Config) GitProtocol(host string) string {
	if hc, ok := c.hosts[normalizeHost(host)]; ok && hc.GitProtocol != "" {
		return hc.GitProtocol
	}
	if c.general.GitProtocol != "" {
		return c.general.GitProtocol
	}
	return "https"
}

// SetGitProtocol stores the preferred clone protocol for host.
func (c *Config) SetGitProtocol(host, protocol string) {
	host = normalizeHost(host)
	if hc, ok := c.hosts[host]; ok {
		hc.GitProtocol = protocol
		return
	}
	c.hosts[host] = &HostConfig{GitProtocol: protocol}
}

func (c *Config) Editor() string {
	if v := os.Getenv("GV_EDITOR"); v != "" {
		return v
	}
	if c.general.Editor != "" {
		return c.general.Editor
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	return os.Getenv("EDITOR")
}

func (c *Config) Pager() string {
	if v := os.Getenv("GV_PAGER"); v != "" {
		return v
	}
	if c.general.Pager != "" {
		return c.general.Pager
	}
	return os.Getenv("PAGER")
}

// Write persists both files, keeping credentials at 0600 in a 0700 directory.
func (c *Config) Write() error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}

	generalData, err := yaml.Marshal(c.general)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(generalData)) != "{}" {
		if err := writeFile(filepath.Join(c.dir, "config.yml"), generalData, 0o600); err != nil {
			return err
		}
	}

	hostsData, err := yaml.Marshal(c.hosts)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(c.dir, "hosts.yml"), hostsData, 0o600)
}

// writeFile writes atomically so an interrupted run cannot leave a truncated
// hosts.yml behind.
func writeFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Host resolves the host to operate against: an explicit value, else GV_HOST,
// else the default instance.
func Host(explicit string) string {
	if explicit != "" {
		return normalizeHost(explicit)
	}
	if v := os.Getenv("GV_HOST"); v != "" {
		return normalizeHost(v)
	}
	return DefaultHost
}

func normalizeHost(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimSuffix(h, "/")
	if h == "" {
		return DefaultHost
	}
	return h
}
