package scmapanalyzer

import "embed"

//go:embed hd_units/*.qoi hd_units/*.mask.png hd_units/*.shadow.png
var hdQoiFiles embed.FS
