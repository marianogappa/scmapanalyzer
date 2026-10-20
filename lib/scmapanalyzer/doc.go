// Package scmapanalyzer provides a small client over [replaymap.Analyze] with
// embedded ladder-map JSON. Use [NewClient] and [Client.Analyze]; optional
// [WithMapName] enables cache hits without reading the replay file.
//
// Map image helpers return bytes in memory: [MapImageJPEGFromReplayFile] and
// [MapImageJPEGFromScrepReplay] render a downscaled JPEG sized for on-screen display,
// and [MapImagePNGCropFromReplayFile] renders a native-resolution tile-rect crop for
// compositing. [UnitOrBuildingImagePNG] returns unit sprites (HD sprites are
// hd_units/u<ID>.qoi with display names in [hdSpriteUnitNames]). [UnitOrBuildingImage] tints a
// sprite with a player's team color and selects variants such as [UnitVariantBurrowed];
// [UnitOrBuildingImageLayers] returns the untinted sprite and its team-color mask
// (hd_units/u<ID>.mask.png) for callers that tint on their own. Sprites are at 2 pixels per map
// pixel; [UnitImageAnchor] says where the unit's position falls in each, and
// [UnitImageOptions.Shadow] / [UnitShadowImage] return the shadow drawn under it. Sprites, masks,
// shadows and anchors are baked by tools/hdbake from a StarCraft: Remastered install.
//
// [CreepTexturePNG], [CreepEdgesPNG] and [CreepEdgeFrame] draw zerg creep like the game.
package scmapanalyzer
