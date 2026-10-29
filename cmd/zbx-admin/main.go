package main

import (
	"errors"
	"os"

	"zbx-admin/internal/cli"
	"zbx-admin/internal/resolve"
)

func main() {
	if err := cli.Execute(); err != nil {
		var amb *resolve.AmbiguousError
		if errors.As(err, &amb) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
