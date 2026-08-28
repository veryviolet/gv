// Package build carries values stamped into the binary at link time.
package build

import "runtime/debug"

// Version is the released version of gv. Overridden via -ldflags at build time.
var Version = "dev"

// Date is the build date in ISO 8601. Overridden via -ldflags at build time.
var Date = ""

func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 && Version == "dev" {
				Version = "dev-" + s.Value[:7]
			}
		case "vcs.time":
			if Date == "" {
				Date = s.Value
			}
		}
	}
}
