.PHONY: replay-map-analyzer replay-map-gallery gallery-assets publish-gallery publish-maps

OUTPUT_DIR ?= output
REPLAYS_DIR ?= replays
ONLY_RUN_MAP ?=
GALLERY_MAX_DIM ?= 1536
GALLERY_TAG ?=

NIGHTLY_MAPS_DIR := $(OUTPUT_DIR)/replaymap-analyzer
GALLERY_DIR := $(NIGHTLY_MAPS_DIR)/gallery
LIB_MAPS_DIR := lib/scmapanalyzer/cache/maps

# Analyze maps into output/replaymap-analyzer/*.json + *-overlay.png from replays only.
# Optional: make replay-map-analyzer ONLY_RUN_MAP=dominator_se
replay-map-analyzer:
	rm -rf "$(NIGHTLY_MAPS_DIR)"
	mkdir -p "$(NIGHTLY_MAPS_DIR)"
	go run ./cmd/replaymapanalyzer \
		-replays-dir "$(REPLAYS_DIR)" \
		-output-dir "$(OUTPUT_DIR)" \
		$(if $(ONLY_RUN_MAP),-only-run-map "$(ONLY_RUN_MAP)")
	@$(MAKE) --no-print-directory replay-map-gallery

# Build an HTML gallery of the overlay PNGs for easy browser review.
# Output: $(NIGHTLY_MAPS_DIR)/index.html (gitignored).
replay-map-gallery:
	@out="$(NIGHTLY_MAPS_DIR)/index.html"; \
	{ \
		printf '<!doctype html>\n<meta charset="utf-8">\n<title>Replay map overlays</title>\n'; \
		printf '<style>body{background:#111;color:#eee;font-family:sans-serif;margin:16px}h1{margin:0 0 16px}.m{margin:24px 0}.m h2{margin:0 0 8px;font-size:16px;font-weight:500}img{max-width:100%%;height:auto;border:1px solid #333;background:#000}</style>\n'; \
		printf '<h1>Replay map overlays</h1>\n'; \
		for f in "$(NIGHTLY_MAPS_DIR)"/*-overlay.png; do \
			[ -f "$$f" ] || continue; \
			name=$$(basename "$$f" -overlay.png); \
			printf '<div class="m"><h2>%s</h2><img src="%s" alt="%s"></div>\n' "$$name" "$$(basename "$$f")" "$$name"; \
		done; \
	} > "$$out"; \
	echo "Wrote: $$out"

# Build downscaled overlays under $(GALLERY_DIR) for attaching to a GitHub
# release. Source files come from a prior `make replay-map-analyzer` run.
# Override max dimension: make gallery-assets GALLERY_MAX_DIM=2048
gallery-assets:
	@command -v sips >/dev/null 2>&1 || { echo "sips not found (macOS-only tool)"; exit 1; }
	@set -e; \
	rm -rf "$(GALLERY_DIR)"; \
	mkdir -p "$(GALLERY_DIR)"; \
	count=0; \
	for f in "$(NIGHTLY_MAPS_DIR)"/*-overlay.png; do \
		[ -f "$$f" ] || continue; \
		sips -Z $(GALLERY_MAX_DIM) "$$f" --out "$(GALLERY_DIR)/$$(basename "$$f")" >/dev/null; \
		count=$$((count+1)); \
	done; \
	if [ $$count -eq 0 ]; then echo "No overlay PNGs found in $(NIGHTLY_MAPS_DIR)"; exit 1; fi; \
	echo "Wrote $$count overlays to $(GALLERY_DIR)"; \
	du -sh "$(GALLERY_DIR)"

# Upload $(GALLERY_DIR)/*.png as assets on a GitHub release. Requires `gh`.
# Usage: make publish-gallery GALLERY_TAG=gallery-v1
publish-gallery:
	@command -v gh >/dev/null 2>&1 || { echo "gh CLI not found"; exit 1; }
	@if [ -z "$(GALLERY_TAG)" ]; then echo "GALLERY_TAG is required, e.g. make publish-gallery GALLERY_TAG=gallery-v1"; exit 1; fi
	@set -e; \
	if ! ls "$(GALLERY_DIR)"/*-overlay.png >/dev/null 2>&1; then \
		echo "No gallery assets in $(GALLERY_DIR); run 'make gallery-assets' first"; exit 1; \
	fi; \
	if gh release view "$(GALLERY_TAG)" >/dev/null 2>&1; then \
		gh release upload "$(GALLERY_TAG)" --clobber "$(GALLERY_DIR)"/*-overlay.png; \
	else \
		gh release create "$(GALLERY_TAG)" \
			--title "Map overlay gallery $(GALLERY_TAG)" \
			--notes "Regenerated debug overlays for README gallery." \
			"$(GALLERY_DIR)"/*-overlay.png; \
	fi; \
	echo "Assets available at https://github.com/marianogappa/scmapanalyzer/releases/download/$(GALLERY_TAG)/<name>-overlay.png"

# Promote nightly map analysis into lib/scmapanalyzer/cache/maps.
publish-maps:
	rm -rf "$(LIB_MAPS_DIR)"
	mkdir -p "$(LIB_MAPS_DIR)"
	@set -e; found=0; \
	for f in "$(NIGHTLY_MAPS_DIR)"/*.json; do \
		if [ -f "$$f" ]; then cp "$$f" "$(LIB_MAPS_DIR)/"; found=1; fi; \
	done; \
	if [ "$$found" -eq 0 ]; then echo "No map JSON files found in $(NIGHTLY_MAPS_DIR)"; exit 1; fi
	@echo "Published maps to $(LIB_MAPS_DIR)"
