# scmapanalyzer

- ⚠️ WARNING: this is a work in progress! Still working in the algorithm, and API is not stable yet.
- `scmapanalyzer` is a Go library that parses StarCraft: Brood War replays and provides:
    - All "base" approximate polygons.
    - Distinction between: "starting locations", "natural expansions" and "expansions".
    - Naming convention for bases, using o'clock notation.
- It also ships with developer tools to debug and evolve the internal algorithms.

## Example debug overlay to visualize what it provides

![Dominator SE debug overlay — starts, expansions, natural paths, wall (red) and ramp (purple) tile borders](docs/sample-dominator-overlay.png)

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

The below tools are used to improve the map analyzer's accuracy. They're meant for contributors, not library consumers.

### Introduction

The map analyzer currently doesn't know tile-tagging information (e.g. which tiles are walls/ramps). The "tile-tagger" tool solves this problem:

- It takes a map image overlayed with red over walls and purple over ramps (examples in `sample-map-masks/`)
- Using a `replays/` folder, it looks for replays with the same map name (which contain tileset tile ids for the map)
- With these, it maps tile ids (on the map's tileset) to wall/ramp information. The output is in `output/tagged-tilesets/`.

Once the tile-tags are available, the map analyzer can be used to analyze replays:
- It takes a `replays/` folder and uses the `output/tagged-tilesets/` folder to look up tile-tagging information for each replay's map's tileset.
- This is enough to produce a JSON file with the map geometry for each replay's map. However, to see the computed geometry, you need the map image, which it will look for in `map-images/` (e.g. `map-images/dominator_se.jpg`).

### Just use the Makefile targets

- Use the `Makefile` targets as the entrypoint (`tile-tagger`, `replay-map-analyzer`, `publish-tilesets`, `publish-maps`). They have comments on how they work.
- `output/tagged-tilesets` is the "nightly" tag repo used by the CLI tools.
- `lib/scmapanalyzer/cache/{tilesets,maps}` is the "stable" cache used by library consumers.
- `tile-tagger` and `replay-map-analyzer` support `ONLY_RUN_MAP=...` for focused runs.

```bash
make tile-tagger
make replay-map-analyzer
make publish-tilesets
make publish-maps
```

License: [MIT](LICENSE).

## Credits

This project would have been impossible without the invaluable work of [@icza](https://github.com/icza) on [screp](https://github.com/icza/screp), a Go library for parsing StarCraft: Brood War replays, and his online help.