//go:build windows

// The samp-injector command launches GTA and injects one caller-supplied DLL.
package main

import (
	"os"

	"git.justmichael.xyz/omp-tools/samp-injector/internal/app"
	"git.justmichael.xyz/omp-tools/samp-injector/internal/exitcode"
)

func main() {
	code := app.Run(os.Args[1:], os.Stderr)
	if code != exitcode.Success {
		os.Exit(code)
	}
}
