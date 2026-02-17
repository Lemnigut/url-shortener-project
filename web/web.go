package web

import "embed"

//go:embed static/index.html
var StaticFS embed.FS
