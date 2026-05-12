package cli

import "runtime/debug"

// Version is stamped via ldflags at release-build time:
//
//	go build -ldflags="-X github.com/chonalchendo/anvil/internal/cli.Version=v0.1.0"
//
// When empty (the default for `go build` / `go install ./cmd/anvil` from a
// working tree) the value is synthesised from `runtime/debug.ReadBuildInfo`.
var Version string

func resolveVersion() string {
	info, _ := debug.ReadBuildInfo()
	return formatVersion(Version, info)
}

func formatVersion(ldflag string, info *debug.BuildInfo) string {
	if ldflag != "" {
		return ldflag
	}
	if info == nil {
		return "dev"
	}
	if info.Main.Sum != "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	var rev string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if modified {
		return "dev-" + rev + "-dirty"
	}
	return "dev-" + rev
}
