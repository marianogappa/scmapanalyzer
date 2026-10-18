package scmapanalyzer

import "embed"

//go:embed hd_units/*.qoi hd_units/*.mask.png
var hdQoiFiles embed.FS
