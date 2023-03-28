package main

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"time"
)

// Version is the version of this program, set at build time with -ldflags.
var Version string

type versionInfo struct {
	Revision string
	Modified bool
	Time     time.Time
}

func getVersion() (string, error) {
	var vi versionInfo
	version := "(unknown)"

	if Version != "" {
		return Version, nil
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				vi.Revision = setting.Value
			case "vcs.time":
				t, err := time.Parse(time.RFC3339, setting.Value)
				if err != nil {
					return version, err
				}
				vi.Time = t
			case "vcs.modified":
				v, err := strconv.ParseBool(setting.Value)
				if err != nil {
					return version, err
				}
				vi.Modified = v
			}
		}

		version = fmt.Sprintf("v0.0.0-%s-%s", vi.Time.Format("20060102150405"), vi.Revision)
		if vi.Modified {
			version = fmt.Sprintf("%s+dirty", version)
		}
	}

	return version, nil
}
