package replaymap

import (
	"reflect"
	"testing"
)

func maskFrom(rows ...string) (int, int, []bool) {
	w, h := len(rows[0]), len(rows)
	m := make([]bool, w*h)
	for y, r := range rows {
		for x, c := range r {
			m[y*w+x] = c == '#'
		}
	}
	return w, h, m
}

func TestMaskOutline(t *testing.T) {
	tests := []struct {
		name string
		rows []string
		want []TilePoint
	}{
		{
			name: "square",
			rows: []string{"....", ".##.", ".##.", "...."},
			want: []TilePoint{{1, 1}, {3, 1}, {3, 3}, {1, 3}},
		},
		{
			name: "concave L keeps its notch",
			rows: []string{"###.....", "###.....", "###.....", "########", "########", "########"},
			want: []TilePoint{{0, 0}, {3, 0}, {3, 3}, {8, 3}, {8, 6}, {0, 6}},
		},
		{
			name: "only the largest component, diagonal contact does not join",
			rows: []string{"##..", "##..", "..#."},
			want: []TilePoint{{0, 0}, {2, 0}, {2, 2}, {0, 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, m := maskFrom(tt.rows...)
			got := maskOutline(w, h, m)
			if !sameCycle(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func sameCycle(a, b []TilePoint) bool {
	if len(a) != len(b) {
		return false
	}
	for shift := range a {
		if reflect.DeepEqual(append(append([]TilePoint{}, a[shift:]...), a[:shift]...), b) {
			return true
		}
	}
	return false
}

func TestTerrainAreasSplitAtChoke(t *testing.T) {
	// Two 20x20 rooms joined by a 4-wide corridor: the watershed must keep
	// them apart and report frontier cells inside the corridor.
	w, h := 60, 22
	data := make([]uint8, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			room := x < 21 || x >= 39
			corridor := y >= 9 && y < 13
			if room || corridor {
				data[y*w+x] = 1
			}
		}
	}
	ta := computeTerrainAreas(w, h, data, make([]uint8, w*h))
	left, right := ta.Area[10*w+10], ta.Area[10*w+50]
	if left < 0 || right < 0 || left == right {
		t.Fatalf("rooms not separated: left=%d right=%d", left, right)
	}
	chokes := extractChokes(ta)
	if len(chokes) != 1 {
		t.Fatalf("want 1 choke, got %d", len(chokes))
	}
	c := chokes[0].seg
	if c.Center.X < 21 || c.Center.X >= 39 || c.Center.Y < 9 || c.Center.Y >= 13 {
		t.Errorf("choke center %v not in corridor", c.Center)
	}
	if c.Width < 3.5 || c.Width > 5 {
		t.Errorf("choke width %.1f, want ~4", c.Width)
	}
}
