package version

import (
	"runtime"
	"runtime/debug"
)

// These variables are intended to be overridden via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info represents version metadata for the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Module    string `json:"module"`
}

// Get returns version metadata for the running binary.
func Get() Info {
	info := Info{
		Version:   stripV(Version),
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.Module = buildInfo.Main.Path
		if info.Version == "dev" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			info.Version = stripV(buildInfo.Main.Version)
		}
	}

	return info
}

// stripV removes a leading "v" from a version string (e.g. "v1.2.0" → "1.2.0").
func stripV(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}