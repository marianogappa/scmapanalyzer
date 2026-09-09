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

// RenderMapJPEGFromMetadata converts replay-side metadata to downscaled JPEG bytes.
func RenderMapJPEGFromMetadata(meta *model.MapMetadata, opts RenderOptions, jopts JPEGOptions) ([]byte, error) {
	md, err := MapDataFromMetadata(meta)
	if err != nil {
		return nil, err
	}
	return RenderMapJPEG(md, opts, jopts)
}

// RenderMapPNGCropFromMetadata converts replay-side metadata to PNG bytes of the
// given tile rect at native 32 px per tile.
func RenderMapPNGCropFromMetadata(meta *model.MapMetadata, opts RenderOptions, r TileRect) ([]byte, error) {
	md, err := MapDataFromMetadata(meta)
	if err != nil {
		return nil, err
	}
	return RenderMapPNGCrop(md, opts, r)
}
