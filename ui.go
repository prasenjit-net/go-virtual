package govirtual

import "embed"

//go:embed all:ui/dist
var EmbeddedUI embed.FS

//go:embed all:docs
var EmbeddedDocs embed.FS
