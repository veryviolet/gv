package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// PackageOwner identifies whose packages are addressed: a user or an
// organization. The API keeps the two under different path prefixes but the
// rest of the request is identical.
type PackageOwner struct {
	Name  string
	IsOrg bool
}

func (o PackageOwner) path() string {
	if o.IsOrg {
		return "/orgs/" + url.PathEscape(o.Name)
	}
	return "/users/" + url.PathEscape(o.Name)
}

// packagePath builds the common ".../packages/{type}/{name}" prefix.
func packagePath(owner PackageOwner, pkgType, name string) string {
	return fmt.Sprintf("%s/packages/%s/%s",
		owner.path(), url.PathEscape(pkgType), escapeFilePath(name))
}

// ListPackageVersions lists the versions of one package.
//
// The API has no endpoint for enumerating packages themselves, so the package
// name has to be known; `gv package list` gets it from the registry catalogue.
func ListPackageVersions(ctx context.Context, c *Client, owner PackageOwner, pkgType, name string, limit int) ([]PackageVersion, error) {
	return ListAll[PackageVersion](ctx, c, packagePath(owner, pkgType, name)+"/versions", limit)
}

// DeletePackage removes a package with all of its versions.
func DeletePackage(ctx context.Context, c *Client, owner PackageOwner, pkgType, name string) error {
	return c.REST(ctx, http.MethodDelete, packagePath(owner, pkgType, name), nil, nil)
}

// DeletePackageVersion removes a single version of a package.
func DeletePackageVersion(ctx context.Context, c *Client, owner PackageOwner, pkgType, name, versionID string) error {
	path := packagePath(owner, pkgType, name) + "/versions/" + url.PathEscape(versionID)
	return c.REST(ctx, http.MethodDelete, path, nil, nil)
}

// PackageType returns the version's type, falling back to the requested one
// when the API leaves the metadata empty.
func (v *PackageVersion) PackageType(fallback string) string {
	if v.Metadata.PackageType != "" {
		return v.Metadata.PackageType
	}
	return fallback
}
