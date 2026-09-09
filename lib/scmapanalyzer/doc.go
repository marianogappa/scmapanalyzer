// Package scmapanalyzer provides a small client over [replaymap.Analyze] with
// embedded ladder-map JSON. Use [NewClient] and [Client.Analyze]; optional
// [WithMapName] enables cache hits without reading the replay file.
//
// Map image helpers return bytes in memory: [MapImageJPEGFromReplayFile] and
// [MapImageJPEGFromScrepReplay] render a downscaled JPEG sized for on-screen display,
// and [MapImagePNGCropFromReplayFile] renders a native-resolution tile-rect crop for
// compositing. [UnitOrBuildingImagePNG] returns unit sprites (HD sprites are
// hd_units/u<ID>.qoi with display names in [hdSpriteUnitNames]).
package scmapanalyzer
