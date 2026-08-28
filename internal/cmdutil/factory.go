package cmdutil

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/browser"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/config"
	"gitverse.ru/veryviolet/gv/internal/git"
	"gitverse.ru/veryviolet/gv/internal/gvrepo"
	"gitverse.ru/veryviolet/gv/internal/iostreams"
)

// Factory carries the dependencies every command needs. Fields are functions
// so that nothing — config, network, git — is touched until a command actually
// asks for it, and so tests can substitute stubs.
type Factory struct {
	IOStreams *iostreams.IOStreams
	Browser   browser.Browser
	GitClient *git.Client

	Config     func() (*config.Config, error)
	HTTPClient func() (*api.Client, error)
	BaseRepo   func() (gvrepo.Interface, error)
	Remotes    func() (git.RemoteSet, error)

	// Host is the GitVerse instance commands operate against.
	Host func() string

	// ExecutableName is the name gv was invoked as, for help text.
	ExecutableName string
}

// NewFactory wires the real implementations together.
func NewFactory() *Factory {
	io := iostreams.System()
	gitClient := &git.Client{Stderr: io.ErrOut}

	f := &Factory{
		IOStreams:      io,
		GitClient:      gitClient,
		Browser:        browser.New("", io.Out, io.ErrOut),
		ExecutableName: "gv",
	}

	var (
		cfgOnce sync.Once
		cfg     *config.Config
		cfgErr  error
	)
	f.Config = func() (*config.Config, error) {
		cfgOnce.Do(func() { cfg, cfgErr = config.Load() })
		return cfg, cfgErr
	}

	f.Host = func() string { return config.Host("") }

	f.HTTPClient = func() (*api.Client, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		host := f.Host()
		token, _ := cfg.Token(host)
		return api.NewClientForHost(host, token, "gv/"+build.Version)
	}

	f.Remotes = func() (git.RemoteSet, error) {
		return gitClient.Remotes(context.Background())
	}

	f.BaseRepo = func() (gvrepo.Interface, error) {
		remotes, err := f.Remotes()
		if err != nil {
			return nil, ErrNoRepo
		}
		return baseRepoFromRemotes(remotes, f.Host())
	}

	return f
}

// OverrideHost pins the GitVerse instance, as --hostname does.
func (f *Factory) OverrideHost(host string) {
	if host == "" {
		return
	}
	f.Host = func() string { return config.Host(host) }
}

// OverrideBaseRepo pins the repository, as --repo OWNER/REPO does.
func (f *Factory) OverrideBaseRepo(nwo string) error {
	repo, err := gvrepo.FromFullNameWithHost(nwo, f.Host())
	if err != nil {
		return FlagErrorf("argument error: %v", err)
	}
	f.BaseRepo = func() (gvrepo.Interface, error) { return repo, nil }
	return nil
}

// baseRepoFromRemotes picks the repository a bare command should act on: the
// first remote pointing at the current host, in upstream/gitverse/origin order.
func baseRepoFromRemotes(remotes git.RemoteSet, host string) (gvrepo.Interface, error) {
	var offHost []string
	for _, r := range remotes {
		u := r.FetchURL
		if u == nil {
			u = r.PushURL
		}
		if u == nil {
			continue
		}
		if !strings.EqualFold(u.Hostname(), host) {
			offHost = append(offHost, fmt.Sprintf("%s (%s)", r.Name, u.Hostname()))
			continue
		}
		repo, err := gvrepo.FromURL(u)
		if err != nil {
			continue
		}
		return repo, nil
	}
	if len(offHost) > 0 {
		return nil, fmt.Errorf("none of the git remotes point to %s: %s", host, strings.Join(offHost, ", "))
	}
	return nil, ErrNoRepo
}
