package clikit

import "runtime/debug"

// BuildVersion reports the version Go embedded in the binary: the module
// version for `go install module@v1.2.3`, or the pseudo-version Go derives
// from the repository for `go build` inside a checkout (which already carries
// the commit, the date and a +dirty marker). Source without VCS information —
// a tarball, or -buildvcs=false — yields "unknown".
//
// Prefer a linker-supplied version when the build sets one:
//
//	var version string // -ldflags "-X main.version=$(git describe --tags)"
//
//	if version == "" {
//		version = clikit.BuildVersion()
//	}
//	clikit.New(root, clikit.WithVersion(version)).Run(ctx)
func BuildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "unknown"
	}
	return info.Main.Version
}
