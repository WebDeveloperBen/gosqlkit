package version

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
	BuiltBy = "source"
)

var (
	once sync.Once
	info Info
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	BuiltBy string `json:"built_by,omitempty"`
	Go      string `json:"go"`
}

func Get() Info {
	once.Do(resolve)
	return info
}

func String() string {
	i := Get()
	parts := []string{"pgkit " + i.Version}
	if i.Commit != "" {
		parts = append(parts, "commit "+i.Commit)
	}
	if i.Date != "" {
		parts = append(parts, "built "+i.Date)
	}
	if i.BuiltBy != "" {
		parts = append(parts, "by "+i.BuiltBy)
	}
	parts = append(parts, i.Go)
	return strings.Join(parts, " ")
}

func resolve() {
	info = Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		BuiltBy: BuiltBy,
		Go:      runtime.Version(),
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			case "vcs.modified":
				if setting.Value == "true" && info.Commit != "" && !strings.HasSuffix(info.Commit, "-dirty") {
					info.Commit += "-dirty"
				}
			}
		}
	}
}
