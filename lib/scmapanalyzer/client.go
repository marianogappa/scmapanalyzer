package scmapanalyzer

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/marianogappa/scmapanalyzer/internal/replay"
	"github.com/marianogappa/scmapanalyzer/internal/tiletags"
	"github.com/marianogappa/scmapanalyzer/replaymap"
)

//go:embed cache/maps/*.json cache/tilesets/*.json
var cacheData embed.FS

// fuzzyMatchMin is the minimum [mapNameSimilarity] score to accept a hint-only
// match without parsing the replay.
const fuzzyMatchMin = 0.82

// Client loads embedded ladder map analyses and per-tileset wall/ramp tag JSON.
// It is safe for concurrent use.
type Client struct {
	tilesets map[string]*tiletags.TileSetTags

	mu            sync.RWMutex
	byExactKey    map[string]*replaymap.Result
	embeddedOrder []*replaymap.Result

	dynMu   sync.RWMutex
	dynamic map[string]*replaymap.Result
}

// NewClient reads all JSON under lib/scmapanalyzer/cache into memory.
func NewClient() (*Client, error) {
	c := &Client{
		tilesets:      map[string]*tiletags.TileSetTags{},
		byExactKey:    map[string]*replaymap.Result{},
		dynamic:       map[string]*replaymap.Result{},
		embeddedOrder: nil,
	}

	tileEntries, err := fs.Glob(cacheData, "cache/tilesets/*.json")
	if err != nil {
		return nil, err
	}
	for _, name := range tileEntries {
		b, err := cacheData.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var tags tiletags.TileSetTags
		if err := json.Unmarshal(b, &tags); err != nil {
			return nil, err
		}
		if tags.TileSetKey == "" {
			return nil, errors.New("embedded tileset JSON missing tileset_key")
		}
		c.tilesets[tags.TileSetKey] = &tags
	}

	mapEntries, err := fs.Glob(cacheData, "cache/maps/*.json")
	if err != nil {
		return nil, err
	}
	for _, name := range mapEntries {
		b, err := cacheData.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var res replaymap.Result
		if err := json.Unmarshal(b, &res); err != nil {
			return nil, err
		}
		k := NormalizeMapKey(res.MapName)
		if k == "" {
			continue
		}
		rp := new(replaymap.Result)
		*rp = res
		c.byExactKey[k] = rp
		c.embeddedOrder = append(c.embeddedOrder, rp)
	}
	return c, nil
}

// Analyze returns polygon summaries for starting locations and expansions. If
// [WithMapName] matches a cached entry, the replay file is not read. Otherwise
// the replay is parsed, tile tags are resolved from the embedded tileset
// repository, and [replaymap.Analyze] runs; the result is stored in an
// in-memory cache keyed by the normalized replay map name.
func (c *Client) Analyze(replayPath string, opts ...Option) (*replaymap.Result, error) {
	if replayPath == "" {
		return nil, errors.New("replay path is required")
	}
	var o options
	for _, fn := range opts {
		fn(&o)
	}

	if o.mapNameHint != "" {
		if r := c.lookupByMapNameHint(o.mapNameHint); r != nil {
			return cloneResult(r, replayPath)
		}
	}

	meta, err := replay.ParseMapMetadata(replayPath)
	if err != nil {
		return nil, err
	}
	key := NormalizeMapKey(meta.MapName)

	c.dynMu.RLock()
	if dr, ok := c.dynamic[key]; ok {
		c.dynMu.RUnlock()
		return cloneResult(dr, replayPath)
	}
	c.dynMu.RUnlock()

	c.mu.RLock()
	if er, ok := c.byExactKey[key]; ok {
		c.mu.RUnlock()
		return cloneResult(er, replayPath)
	}
	c.mu.RUnlock()

	tags, ok := c.tilesets[meta.TilesetKey]
	if !ok || tags == nil {
		return nil, fmt.Errorf("no embedded tile tags for tileset_key %q", meta.TilesetKey)
	}
	out, err := replaymap.Analyze(meta, tags)
	if err != nil {
		return nil, err
	}
	storeKey := NormalizeMapKey(out.Result.MapName)
	stored, err := cloneResult(out.Result, out.Result.ReplayPath)
	if err != nil {
		return nil, err
	}
	c.dynMu.Lock()
	c.dynamic[storeKey] = stored
	c.dynMu.Unlock()

	return cloneResult(out.Result, replayPath)
}

func (c *Client) lookupByMapNameHint(hint string) *replaymap.Result {
	nk := NormalizeMapKey(hint)

	c.dynMu.RLock()
	for _, dr := range c.dynamic {
		if NormalizeMapKey(dr.MapName) == nk {
			c.dynMu.RUnlock()
			return dr
		}
	}
	c.dynMu.RUnlock()

	c.mu.RLock()
	if r, ok := c.byExactKey[nk]; ok {
		c.mu.RUnlock()
		return r
	}
	c.mu.RUnlock()

	return c.fuzzyPickEmbeddedAndDynamic(hint)
}

func (c *Client) fuzzyPickEmbeddedAndDynamic(name string) *replaymap.Result {
	var best *replaymap.Result
	bestScore := 0.0

	c.mu.RLock()
	for _, cand := range c.embeddedOrder {
		s := mapNameSimilarity(name, cand.MapName)
		if s > bestScore {
			bestScore = s
			best = cand
		}
	}
	c.mu.RUnlock()

	c.dynMu.RLock()
	for _, dr := range c.dynamic {
		s := mapNameSimilarity(name, dr.MapName)
		if s > bestScore {
			bestScore = s
			best = dr
		}
	}
	c.dynMu.RUnlock()

	if bestScore >= fuzzyMatchMin {
		return best
	}
	return nil
}

func cloneResult(r *replaymap.Result, replayPath string) (*replaymap.Result, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var out replaymap.Result
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	out.ReplayPath = replayPath
	return &out, nil
}
