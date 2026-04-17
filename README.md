# scmapanalyzer

Go library for **ladder-map geometry** from StarCraft: Brood War replays: inferred polygons for each starting location and each natural expansion, plus which expansion is the natural for each start. It ships with **embedded** analyses for common maps and **embedded** per-tileset wall/ramp tag JSON so callers do not need to ship `output/` at runtime.

![Dominator SE debug overlay — starts, expansions, natural paths, wall (red) and ramp (purple) tile borders](docs/sample-dominator-overlay.png)

## Why use this

- **Structured map data** — get clock positions, tile polygons, and `natural_expansion` links without maintaining fragile hand-authored map metadata.
- **Fast repeat lookups** — optional map name hint resolves from an in-memory cache (embedded JSON + anything analyzed earlier in the process) so you can skip replay I/O when you already know the ladder name.
- **Replay-accurate** — geometry is derived from the replay’s tile grid, minerals, geysers, and start locations together with walkability heuristics (walls/ramps from tagged tilesets).
- **Easy debugging** — the same analysis feeds a minimap overlay (see [gallery](docs/output-gallery.md)) for validating regions on real map art.

## What you get

The API returns [`replaymap.Result`](replaymap/types.go): JSON-serializable fields match the shape below.

```go
type TilePoint struct {
    X int `json:"x"`
    Y int `json:"y"`
}

type Result struct {
    ReplayPath string         `json:"replay_path"`
    MapName    string         `json:"map_name"`
    TileSetKey string         `json:"tileset_key"`
    Starts     []BasePolygon  `json:"starting_locations"`
    Expas      []BasePolygon  `json:"expansions"`
}

type BasePolygon struct {
    Name             string      `json:"name"`
    Kind             string      `json:"kind"`              // "start" or "expa"
    Clock            int         `json:"clock"`             // minimap clock hour (1,3,5,…)
    CenterTile       TilePoint   `json:"center_tile"`
    PolygonVertices  []TilePoint `json:"polygon_vertices"`
    NaturalExpansion string      `json:"natural_expansion,omitempty"` // expa name for starts
}
```

## Usage

```go
import (
    "log"

    "github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
)

func run() {
    client, err := scmapanalyzer.NewClient()
    if err != nil {
        log.Fatal(err)
    }

    // Optional: skip parsing when the ladder name matches embedded data.
    res, err := client.Analyze("/path/to/game.rep", scmapanalyzer.WithMapName("Fighting Spirit"))
    if err != nil {
        log.Fatal(err)
    }

    _ = res.Starts
    _ = res.Expas
    _ = res.TileSetKey
}
```

Import path: `github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer`.

## Developers

- **`cmd/replaymapanalyzer`** — CLI: read one replay, write JSON and optional PNG overlay (needs `-map-image` from [`map-images/`](map-images/) for the overlay).
- **`cmd/tiletagger`** — build or extend wall/ramp tile ID lists from a minimap plus [`sample-map-masks/`](sample-map-masks/) (red wall / purple ramp) and a replay search path; writes per-tileset JSON like those under `lib/scmapanalyzer/cache/tilesets/`.
- **Batch** — `make replaymap-analyzer-all` runs [`scripts/replaymap_analyzer_batch.py`](scripts/replaymap_analyzer_batch.py) when `output/map-analysis/matched-maps.json` is present.

```bash
go run ./cmd/replaymapanalyzer \
  -replay replays/30-lIlIIIIIIllllll/MM-268BF7A8-FC4B-11F0-A3DD-FA167B5461B6.rep \
  -tags-repo output/tagged-tilesets \
  -map-image map-images/dominator_se.jpg \
  -out-json output/replaymap-analyzer.json \
  -out-image output/replaymap-analyzer-overlay.png
```

License: [MIT](LICENSE).
