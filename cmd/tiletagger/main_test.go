package main

import (
	"slices"
	"testing"
)

func TestMergeIDsRemovesWalkable(t *testing.T) {
	got := mergeIDs(
		[]uint16{1, 2},
		[]uint16{2, 3},
		[]uint16{2},
	)
	want := []uint16{1, 3}
	if !slices.Equal(got, want) {
		t.Fatalf("mergeIDs result mismatch: got %v want %v", got, want)
	}
}

func TestMergeIDsRemovesWalkableFromCurrentRun(t *testing.T) {
	got := mergeIDs(
		nil,
		[]uint16{5, 6},
		[]uint16{6},
	)
	want := []uint16{5}
	if !slices.Equal(got, want) {
		t.Fatalf("mergeIDs result mismatch: got %v want %v", got, want)
	}
}
