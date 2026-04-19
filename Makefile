.PHONY: tile-tagger replay-map-analyzer publish-tilesets publish-maps ensure-nightly-tilesets

OUTPUT_DIR ?= output
REPLAYS_DIR ?= replays
MAP_IMAGES_DIR ?= map-images
OVERLAYS_DIR ?= sample-map-masks
ONLY_RUN_MAP ?=

# "Nightly" generated assets (used by developer commands):
NIGHTLY_TILESETS_DIR := $(OUTPUT_DIR)/tagged-tilesets
NIGHTLY_MAPS_DIR := $(OUTPUT_DIR)/replaymap-analyzer

# "Stable" library cache (embedded by lib/scmapanalyzer):
LIB_TILESETS_DIR := lib/scmapanalyzer/cache/tilesets
LIB_MAPS_DIR := lib/scmapanalyzer/cache/maps

# Build/extend tags incrementally in output/tagged-tilesets from all
# map/overlay filename matches. A per-run report is written to
# output/tiletagger-result.json (gitignored). Optional:
# make tile-tagger ONLY_RUN_MAP=dominator_se
tile-tagger:
	go run ./cmd/tiletagger \
		-map-images-dir "$(MAP_IMAGES_DIR)" \
		-overlays-dir "$(OVERLAYS_DIR)" \
		-replays-dir "$(REPLAYS_DIR)" \
		-output-dir "$(OUTPUT_DIR)" \
		$(if $(ONLY_RUN_MAP),-only-run-map "$(ONLY_RUN_MAP)")

# Analyze maps into output/replaymap-analyzer/*.json + *-overlay.png.
# Uses output/tagged-tilesets as the source of truth for these commands.
# Optional:
# make replay-map-analyzer ONLY_RUN_MAP=dominator_se
replay-map-analyzer: ensure-nightly-tilesets
	go run ./cmd/replaymapanalyzer \
		-replays-dir "$(REPLAYS_DIR)" \
		-map-images-dir "$(MAP_IMAGES_DIR)" \
		-output-dir "$(OUTPUT_DIR)" \
		$(if $(ONLY_RUN_MAP),-only-run-map "$(ONLY_RUN_MAP)")

ensure-nightly-tilesets:
	@if [ ! -d "$(NIGHTLY_TILESETS_DIR)" ]; then \
		echo "Nightly tilesets dir not found: $(NIGHTLY_TILESETS_DIR)"; \
		echo "Run: make tile-tagger"; \
		exit 1; \
	fi
	@set -e; found=0; \
	for f in "$(NIGHTLY_TILESETS_DIR)"/*.json; do \
		if [ -f "$$f" ]; then found=1; break; fi; \
	done; \
	if [ "$$found" -eq 0 ]; then \
		echo "No nightly tileset JSON files found in $(NIGHTLY_TILESETS_DIR)"; \
		echo "Run: make tile-tagger"; \
		exit 1; \
	fi

# Promote "nightly" tile tags into lib/scmapanalyzer/cache/tilesets.
publish-tilesets:
	@mkdir -p "$(LIB_TILESETS_DIR)"
	@set -e; found=0; \
	for f in "$(NIGHTLY_TILESETS_DIR)"/*.json; do \
		if [ -f "$$f" ]; then cp "$$f" "$(LIB_TILESETS_DIR)/"; found=1; fi; \
	done; \
	if [ "$$found" -eq 0 ]; then echo "No tileset JSON files found in $(NIGHTLY_TILESETS_DIR)"; exit 1; fi
	@echo "Published tilesets to $(LIB_TILESETS_DIR)"

# Promote "nightly" map analysis into lib/scmapanalyzer/cache/maps.
publish-maps:
	@mkdir -p "$(LIB_MAPS_DIR)"
	@set -e; found=0; \
	for f in "$(NIGHTLY_MAPS_DIR)"/*.json; do \
		if [ -f "$$f" ]; then cp "$$f" "$(LIB_MAPS_DIR)/"; found=1; fi; \
	done; \
	if [ "$$found" -eq 0 ]; then echo "No map JSON files found in $(NIGHTLY_MAPS_DIR)"; exit 1; fi
	@echo "Published maps to $(LIB_MAPS_DIR)"
