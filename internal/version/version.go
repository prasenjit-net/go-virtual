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
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.Module = buildInfo.Main.Path
		if info.Version == "dev" && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			info.Version = buildInfo.Main.Version
		}
	}

	return info
}