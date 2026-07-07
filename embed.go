package main

import (
	"embed"
	"io/fs"
	"os"
)

// embeddedFS holds the templates, migrations, and static assets baked into the
// binary so a downloaded release runs standalone with no accompanying files.
//
//go:embed templates migrations static
var embeddedFS embed.FS

// assetFS returns a filesystem for the named asset tree. If a directory of that
// name exists on disk (running from the source tree during development), it is
// used so edits are picked up without recompiling; otherwise the copy embedded
// in the binary is used, so release binaries need nothing on disk.
func assetFS(dir string) fs.FS {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return os.DirFS(dir)
	}
	sub, err := fs.Sub(embeddedFS, dir)
	if err != nil {
		// embeddedFS always contains dir; unreachable at runtime.
		panic(err)
	}
	return sub
}
