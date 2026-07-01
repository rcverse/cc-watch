package main

import (
	"os"

	"github.com/richardchen/cc-watch/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
