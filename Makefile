.PHONY: replay-map-analyzer publish-maps

OUTPUT_DIR ?= output
REPLAYS_DIR ?= replays
ONLY_RUN_MAP ?=

NIGHTLY_MAPS_DIR := $(OUTPUT_DIR)/replaymap-analyzer
LIB_MAPS_DIR := lib/scmapanalyzer/cache/maps

# Analyze maps into output/replaymap-analyzer/*.json + *-overlay.png from replays only.
# Optional: make replay-map-analyzer ONLY_RUN_MAP=dominator_se
replay-map-analyzer:
	go run ./cmd/replaymapanalyzer \
		-replays-dir "$(REPLAYS_DIR)" \
		-output-dir "$(OUTPUT_DIR)" \
		$(if $(ONLY_RUN_MAP),-only-run-map "$(ONLY_RUN_MAP)")

# Promote nightly map analysis into lib/scmapanalyzer/cache/maps.
publish-maps:
	@mkdir -p "$(LIB_MAPS_DIR)"
	@set -e; found=0; \
	for f in "$(NIGHTLY_MAPS_DIR)"/*.json; do \
		if [ -f "$$f" ]; then cp "$$f" "$(LIB_MAPS_DIR)/"; found=1; fi; \
	done; \
	if [ "$$found" -eq 0 ]; then echo "No map JSON files found in $(NIGHTLY_MAPS_DIR)"; exit 1; fi
	@echo "Published maps to $(LIB_MAPS_DIR)"
