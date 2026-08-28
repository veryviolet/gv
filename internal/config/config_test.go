package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadFrom(dir)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	cfg.SetToken(DefaultHost, "veryviolet", "from-file")

	if token, source := cfg.Token(DefaultHost); token != "from-file" || source != SourceFile {
		t.Errorf("stored token = %q from %q", token, source)
	}

	t.Setenv("GITVERSE_TOKEN", "from-gitverse-env")
	if token, source := cfg.Token(DefaultHost); token != "from-gitverse-env" || source != SourceEnvGitVerse {
		t.Errorf("GITVERSE_TOKEN should win over the file, got %q from %q", token, source)
	}

	t.Setenv("GV_TOKEN", "from-gv-env")
	if token, source := cfg.Token(DefaultHost); token != "from-gv-env" || source != SourceEnvGV {
		t.Errorf("GV_TOKEN should win over everything, got %q from %q", token, source)
	}
}

func TestWriteCreatesPrivateFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gv")
	cfg, err := loadFrom(dir)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	cfg.SetToken("gitverse.ru", "veryviolet", "s3cret")
	if err := cfg.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "hosts.yml"))
	if err != nil {
		t.Fatalf("stat hosts.yml: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("hosts.yml mode = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir mode = %o, want 700", perm)
	}

	// A fresh load must see what was written.
	reloaded, err := loadFrom(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if token, _ := reloaded.Token("gitverse.ru"); token != "s3cret" {
		t.Errorf("reloaded token = %q", token)
	}
	if user := reloaded.User("gitverse.ru"); user != "veryviolet" {
		t.Errorf("reloaded user = %q", user)
	}
}

func TestHostNormalisation(t *testing.T) {
	cases := map[string]string{
		"":                     DefaultHost,
		"GitVerse.ru":          "gitverse.ru",
		"https://gitverse.ru/": "gitverse.ru",
		"git.example.com":      "git.example.com",
	}
	for in, want := range cases {
		if got := Host(in); got != want {
			t.Errorf("Host(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGitProtocolDefaultsToHTTPS(t *testing.T) {
	cfg, err := loadFrom(t.TempDir())
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if got := cfg.GitProtocol(DefaultHost); got != "https" {
		t.Errorf("GitProtocol = %q, want https", got)
	}
	cfg.SetGitProtocol(DefaultHost, "ssh")
	if got := cfg.GitProtocol(DefaultHost); got != "ssh" {
		t.Errorf("GitProtocol = %q, want ssh", got)
	}
}

func TestLoadMissingConfigIsNotAnError(t *testing.T) {
	cfg, err := loadFrom(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if len(cfg.Hosts()) != 0 {
		t.Errorf("Hosts() = %v, want empty", cfg.Hosts())
	}
}
