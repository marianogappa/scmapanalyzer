// Package scmapanalyzer provides a small client over [replaymap.Analyze] with
// embedded ladder-map JSON and per-tileset wall/ramp tag data. Use
// [NewClient] and [Client.Analyze]; optional [WithMapName] enables cache hits
// without reading the replay file.
package scmapanalyzer
