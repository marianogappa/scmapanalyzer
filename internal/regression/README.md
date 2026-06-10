# regression — detection characterization harness

Lets you evolve and simplify `replaymap.Analyze` (base/polygon detection)
without silently regressing the maps that already work. Drives the real
pipeline over the `.rep` files in `replays/`, one per unique map.

## Two tiers

**Semantic invariants** — `invariants_test.go`, `TestSemanticInvariants`.
A hand-written table of what is simply right or wrong per map: start count,
expansion count, number of bases in the center circle, every start has a
natural. Never auto-generated. This is the contract; edit it on purpose, in
review.

**Geometry golden** — `golden_test.go`, `TestGoldenGeometry`. Compares each
base's center and polygon against `testdata/golden/<map>.json` with tolerance:
center drift ≤ `maxCenterDistTiles`, polygon overlap `IoU ≥ minIoU`. Absorbs
harmless vertex jitter; catches a region that actually moved or collapsed.

## Workflow

```sh
# See current counts (use to seed/update the invariant table)
go test ./internal/regression -run TestCorpusSummary -v

# Normal run — both tiers must pass
go test ./internal/regression

# After a deliberate, reviewed improvement: re-approve the geometry snapshots
go test ./internal/regression -run TestGoldenGeometry -update
```

When you change detection on purpose: update the affected `expected` entry,
re-run `-update`, and commit the golden diff in the same PR so the decision is
recorded. Review the dropped maps' overlay PNGs before re-approving.

## The 1-4 center bug

`1-4` (and `1-3`) currently detect two bases in the center that should be one.
`TestCenterBaseTargets` encodes the target (`centerBases: 1`) and is skipped.
To work the fix: remove the `t.Skip`, make it pass, then update `1-4` in
`expected` to `centerBases: 1`. The other 18 maps' invariants + geometry guard
the fix against collateral damage.
