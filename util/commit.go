package util

import (
	"fmt"
	"runtime/debug"
)

var Revision = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Println(info)
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value[:7]
			}
		}
	}
	return "unknown"
}()
