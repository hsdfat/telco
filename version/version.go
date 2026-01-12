package version

import (
	"runtime/debug"
	"strings"
)

type BuildInfo struct {
	Project      string            `json:"project"`
	Hash         string            `json:"hash"`
	BuildDate    string            `json:"date"`
	BuildHost    string            `json:"host"`
	GoVersion    string            `json:"go_version"`
	Dependencies map[string]string `json:"dependencies"`
}

type Dependency struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

func GetBuildInfo() BuildInfo {
	buildInfo := BuildInfo{
		Dependencies: make(map[string]string),
	}
	h := ""
	dirty := ""
	LDFlags := make(map[string]string)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				h = setting.Value
				if len(h) > 8 {
					h = h[:8]
				}
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				dirty = "-dirty"
			}
			if setting.Key == "ldflags" {
				// Parse ldflags into map
				LDFlags = parseLDFlags(setting.Value)
			}
		}
		buildInfo.GoVersion = info.GoVersion
		buildInfo.Project = info.Main.Path
		for _, dep := range info.Deps {
			if dep.Replace != nil {
				buildInfo.Dependencies[dep.Replace.Path] = dep.Replace.Version
				continue
			}
			buildInfo.Dependencies[dep.Path] = dep.Version
		}
	}
	buildInfo.Hash = h + dirty
	if date, ok := LDFlags["main.date"]; ok {
		buildInfo.BuildDate = date
	}
	if host, ok := LDFlags["main.host"]; ok {
		buildInfo.BuildHost = host
	}
	return buildInfo
}

func parseLDFlags(ldflags string) map[string]string {
	result := make(map[string]string)
	var isXFlag bool
	parts := strings.Fields(ldflags)

	for _, part := range parts {
		if part == "-X" {
			isXFlag = true
			continue
		}
		if isXFlag {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				result[kv[0]] = strings.Trim(kv[1], "'\"")
			}
			isXFlag = false
		}
	}
	return result
}
