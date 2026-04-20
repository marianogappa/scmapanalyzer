package mapgfx

import "testing"

func TestTilesetAssetFolderFromReplay_embeddedKeys(t *testing.T) {
	keys := []struct {
		key    string
		folder string
	}{
		{"00-badlands", "badlands"},
		{"01-space-platform", "platform"},
		{"03-ashworld", "ashworld"},
		{"04-jungle", "jungle"},
		{"05-desert", "desert"},
		{"06-arctic", "ice"},
		{"07-twilight", "twilight"},
		{"02-install", "install"},
		{"badlands", "badlands"},
		{"platform", "platform"},
		{"00-badlands-missing-era", "badlands"},
	}
	for _, tc := range keys {
		got, err := TilesetAssetFolderFromReplay(tc.key)
		if err != nil {
			t.Fatalf("%q: %v", tc.key, err)
		}
		if got != tc.folder {
			t.Fatalf("%q: got %q want %q", tc.key, got, tc.folder)
		}
	}
}

func TestTilesetAssetFolderFromReplay_unknown(t *testing.T) {
	_, err := TilesetAssetFolderFromReplay("99-not-a-real-tileset-slug")
	if err == nil {
		t.Fatal("expected error")
	}
}
