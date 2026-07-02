package mapgfx

import "embed"

//go:embed data/tilesets/*/*.tspk
//go:embed data/tilesets/*/features.bin
//go:embed data/sprites/*.spr
//go:embed data/sprites/manifest.json
var assets embed.FS
