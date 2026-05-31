package assets

import (
	"embed"
	"io/fs"
)

//go:embed pipelines/*.yaml tools/*.yaml
var FS embed.FS

// ReadFile provides direct access to embedded assets.
func ReadFile(name string) ([]byte, error) {
	return FS.ReadFile(name)
}

// ReadDir returns the entries in a directory inside the embedded FS.
func ReadDir(dir string) ([]fs.DirEntry, error) {
	return FS.ReadDir(dir)
}
