package scmapanalyzer_test

import (
	"log"

	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
)

func ExampleNewClient() {
	client, err := scmapanalyzer.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// WithMapName skips replay parsing when the hint matches embedded data.
	_, _ = client.Analyze("/path/to/replay.rep", scmapanalyzer.WithMapName("Dominator SE"))
}
