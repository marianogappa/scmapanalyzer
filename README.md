# scmapanalyzer

- ⚠️ WARNING: this is a work in progress! Still working in the algorithm, and API is not stable yet.
- `scmapanalyzer` is a Go library that parses StarCraft: Brood War replays and provides:
    - All "base" approximate polygons.
    - Distinction between: "starting locations", "natural expansions" and "expansions".
    - Which base is the natural expansion of a starting location.
    - Whether bases are mineral-only or not.
    - Naming convention for bases, using o'clock notation.
- It also ships with developer tools to debug and evolve the internal algorithms.

## Example debug overlay to visualize what it provides

![Big Game Hunters debug overlay — starts (cyan), expansions, natural paths, solid terrain (red), ramp minitiles (purple)](docs/big-game-hunters-overlay.png)

## Why is this useful?

- This is a library meant to be imported by replay analyzers.
- It allows analyzers to put replay commands' coordinates in the context of the map geometry, example:
    - Building a Hatchery is not the same as _"expanding" to a natural expansion_.
    - Building a Cannon _in your natural_ in a ZvP at a certain timing means defending from a Hydra timing.
    - Building a Bunker _in close proximity to your enemy's natural_ at a certain timing in a TvZ is a Bunker Rush.
    - ... you see the idea. You can also track who "owns" each base in linear time as you parse the replay.

## What you get

The API returns [`replaymap.Result`](replaymap/types.go): JSON-serializable fields match the shape below.

```go
type Result struct {
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

type TilePoint struct {
    X int `json:"x"`
    Y int `json:"y"`
}
```

`TilePoint` values are in **minitile** coordinates (8×8 px steps; four minitiles per map tile). `center_tile` and `polygon_vertices` use the same grid.

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

    // Important! If you pass the map name, and the map exists in the pre-warmed cache, the replay is not parsed.
    // This makes the method return immediately.
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

## Developer tools

### Makefile targets

- `make replay-map-analyzer` — scans `replays/` (override with `REPLAYS_DIR=`), writes `output/replaymap-analyzer/*.json` and `*-overlay.png` (base map is rendered from replay tile data). Optional `ONLY_RUN_MAP=...`.
- `make publish-maps` — copies those JSON files into `lib/scmapanalyzer/cache/maps/` for embedding.

```bash
make replay-map-analyzer
make publish-maps
```

Embedded `lib/scmapanalyzer/cache/maps/*.json` is only refreshed after you run the analyzer and `publish-maps`; until then it may still reflect an older coordinate convention.

License: [MIT](LICENSE).

## Credits

This project would have been impossible without the invaluable work of [@icza](https://github.com/icza) on [screp](https://github.com/icza/screp), a Go library for parsing StarCraft: Brood War replays, and his online help.
