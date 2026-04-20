package mapgfx

import (
	"github.com/marianogappa/scmapanalyzer/internal/model"
)

// RenderMapPNGFromMetadata converts replay-side metadata to PNG bytes (32 px per map tile).
func RenderMapPNGFromMetadata(meta *model.MapMetadata, opts RenderOptions) ([]byte, error) {
	md, err := MapDataFromMetadata(meta)
	if err != nil {
		return nil, err
	}
	return RenderMapPNG(md, opts)
}
