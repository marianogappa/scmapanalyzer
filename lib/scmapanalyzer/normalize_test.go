package scmapanalyzer

import "testing"

func TestNormalizeMapKey_stripsReplayFormatting(t *testing.T) {
	raw := "\x07D\x06o\x07m\x06i\x07n\x06a\x07t\x06o\x07r \x03SE \x052."
	got := NormalizeMapKey(raw)
	want := "dominator-se-2"
	if got != want {
		t.Fatalf("NormalizeMapKey(%q) = %q, want %q", raw, got, want)
	}
}

func TestMapNameSimilarity(t *testing.T) {
	a := "Dominator SE 5.2"
	b := "\x07D\x06o\x07m\x06i\x07n\x06a\x07t\x06o\x07r \x03SE \x052."
	if mapNameSimilarity(a, b) < 0.5 {
		t.Fatalf("expected some similarity between ladder hint and replay map string")
	}
}
