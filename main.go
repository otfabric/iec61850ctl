// SPDX-License-Identifier: MIT

package main

import (
	"github.com/otfabric/iec61850ctl/cmd"
)

// Injected at link time via -ldflags (see Makefile / release workflow):
// -X main.version=... -X main.tag=... -X main.commit=... -X main.buildDate=...
var (
	version   = "dev"
	tag       = ""
	commit    = ""
	buildDate = ""
)

func main() {
	cmd.SetBuildMeta(version, tag, commit, buildDate)
	cmd.Execute()
}
