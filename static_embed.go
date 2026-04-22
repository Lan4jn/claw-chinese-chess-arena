package main

import (
	"embed"
	"io/fs"
)

// Embed static assets so release binaries can serve the web UI
// even when started outside the repository root.
//go:embed static
var embeddedStaticAssets embed.FS

var (
	embeddedStaticFS  = mustEmbeddedSubFS(embeddedStaticAssets, "static")
	embeddedIndexHTML = mustEmbeddedFile("index.html")
)

func mustEmbeddedSubFS(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func mustEmbeddedFile(name string) []byte {
	data, err := fs.ReadFile(embeddedStaticFS, name)
	if err != nil {
		panic(err)
	}
	return data
}
