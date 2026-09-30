// Package pkg implements `gv package`.
package pkg

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/api"
	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// DefaultPackageType is the type container images are filed under. The API
// also accepts generic, debian, rpm, maven, npm, pypi, helm, nuget and go;
// "docker" is rejected as invalid.
const DefaultPackageType = "container"

// NewCmdPackage returns the `gv package` command group.
func NewCmdPackage(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package <command>",
		Short:   "Work with packages and container images",
		Aliases: []string{"packages", "pkg"},
		Long: `List and delete packages published to GitVerse.

Container images live in the registry on the same host as the repositories:
an image is named ` + "`gitverse.ru/<owner>/<image>`" + ` and ` + "`docker login gitverse.ru`" + `
takes the same token gv uses — see ` + "`gv auth docker-login`" + `.

The REST API can only address a package whose name is already known, so
` + "`gv package list`" + ` reads the registry catalogue instead.`,
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdVersions(f))
	cmd.AddCommand(NewCmdDelete(f))

	return cmd
}

// registryClient builds a registry client from the stored credentials.
func registryClient(f *cmdutil.Factory) (*api.RegistryClient, string, error) {
	cfg, err := f.Config()
	if err != nil {
		return nil, "", err
	}
	host := f.Host()
	token, _ := cfg.Token(host)
	if token == "" {
		return nil, "", cmdutil.NewAuthError(errNoToken{host})
	}
	return api.NewRegistryClient(host, cfg.User(host), token, "gv/"+build.Version), host, nil
}

type errNoToken struct{ host string }

func (e errNoToken) Error() string { return "no token found for " + e.host }

// resolveOwner returns the owner to operate on: the argument, or the
// authenticated user when the argument is empty.
func resolveOwner(ctx context.Context, f *cmdutil.Factory, arg string) (string, error) {
	if arg != "" {
		return strings.Trim(arg, "/"), nil
	}
	cfg, err := f.Config()
	if err != nil {
		return "", err
	}
	if user := cfg.User(f.Host()); user != "" {
		return user, nil
	}

	client, err := f.HTTPClient()
	if err != nil {
		return "", err
	}
	me, err := api.CurrentUser(ctx, client)
	if err != nil {
		return "", err
	}
	return me.DisplayName(), nil
}

// splitPackageName accepts "owner/name" or a bare "name", filling the owner
// in from the authenticated account when it is omitted.
func splitPackageName(ctx context.Context, f *cmdutil.Factory, arg string) (owner, name string, err error) {
	arg = strings.Trim(arg, "/")
	if before, after, ok := strings.Cut(arg, "/"); ok {
		return before, after, nil
	}
	owner, err = resolveOwner(ctx, f, "")
	if err != nil {
		return "", "", err
	}
	return owner, arg, nil
}
