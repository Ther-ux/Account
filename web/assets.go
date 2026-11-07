// Package webassets embeds the production frontend into a standalone executable.
package webassets

import "embed"

// Files contains only public web assets, never the private data directory.
//
//go:embed *.html *.css *.js
var Files embed.FS
