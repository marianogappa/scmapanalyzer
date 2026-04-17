package tiletags

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func Save(repoDir string, tags *TileSetTags) (string, error) {
	if tags == nil {
		return "", errors.New("nil tags")
	}
	if tags.TileSetKey == "" {
		return "", errors.New("tileset key is required")
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(repoDir, tags.TileSetKey+".json")
	b, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func LoadByTileSetKey(repoDir string, tileSetKey string) (*TileSetTags, error) {
	if tileSetKey == "" {
		return nil, errors.New("tileset key is required")
	}
	path := filepath.Join(repoDir, tileSetKey+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tags TileSetTags
	if err := json.Unmarshal(b, &tags); err != nil {
		return nil, err
	}
	return &tags, nil
}
