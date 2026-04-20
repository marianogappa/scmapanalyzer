package mapgfx

import (
	"errors"
	"fmt"
	"strings"
)

// blizzardTilesetDirs matches embedded data/tilesets/* directory names.
var blizzardTilesetDirs = map[string]struct{}{
	"badlands": {}, "platform": {}, "install": {}, "ashworld": {},
	"jungle": {}, "desert": {}, "ice": {}, "twilight": {},
}

// slugFromTilesetKey strips a leading "NN-" numeric prefix from replay tileset_key values.
func slugFromTilesetKey(tilesetKey string) string {
	key := strings.TrimSpace(strings.ToLower(tilesetKey))
	if len(key) >= 3 && key[2] == '-' {
		if key[0] >= '0' && key[0] <= '9' && key[1] >= '0' && key[1] <= '9' {
			return key[3:]
		}
	}
	return key
}

// replaySlugToAssetFolder maps slugified repcore tileset names to embedded folder names.
var replaySlugToAssetFolder = map[string]string{
	"badlands":        "badlands",
	"space-platform":  "platform",
	"platform":        "platform",
	"install":         "install",
	"installation":    "install",
	"ashworld":        "ashworld",
	"jungle":          "jungle",
	"desert":          "desert",
	"arctic":          "ice",
	"ice":             "ice",
	"twilight":        "twilight",
	"unknown":         "badlands",
}

// TilesetAssetFolderFromReplay resolves meta.TilesetKey (or a bare Blizzard folder name) to an embedded tileset directory.
func TilesetAssetFolderFromReplay(tilesetKey string) (string, error) {
	key := strings.TrimSpace(strings.ToLower(tilesetKey))
	if key == "" {
		return "", errors.New("empty tileset key")
	}
	if _, ok := blizzardTilesetDirs[key]; ok {
		return key, nil
	}
	slug := slugFromTilesetKey(key)
	slug = strings.TrimSuffix(slug, "-missing-era")
	if folder, ok := replaySlugToAssetFolder[slug]; ok {
		return folder, nil
	}
	return "", fmt.Errorf("unsupported tileset for map assets: %q (slug %q)", tilesetKey, slug)
}
