// Package dashboard provides the embedded web dashboard for uniteOS.
package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var staticFiles embed.FS

// GetStaticFS returns the embedded static files as an fs.FS.
func GetStaticFS() (fs.FS, error) {
	return fs.Sub(staticFiles, "static")
}
