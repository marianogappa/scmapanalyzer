package replaymap

import "math"

// rayAngleDegrees returns the map ray angle in [0, 360), same basis as [oclock]
// (Atan2 from map center in pixel space).
func rayAngleDegrees(widthTiles int, heightTiles int, x int, y int) float64 {
	mapWidth := float64(widthTiles * 32)
	mapHeight := float64(heightTiles * 32)
	centerX := mapWidth / 2
	centerY := mapHeight / 2
	angle := math.Atan2(float64(y)-centerY, float64(x)-centerX) * 180.0 / math.Pi
	if angle < 0 {
		angle += 360
	}
	return angle
}

// oclockFloat returns a continuous clock coordinate in (0, 12], aligned so that
// each 30° of [rayAngleDegrees] is one hour and hour 3 matches the east sector
// used by [oclock] (0° → 3, 90° → 6, 270° → 12).
func oclockFloat(widthTiles int, heightTiles int, x int, y int) float64 {
	ang := rayAngleDegrees(widthTiles, heightTiles, x, y)
	v := 3.0 + ang/30.0
	for v > 12 {
		v -= 12
	}
	for v <= 0 {
		v += 12
	}
	return v
}

// normDial12 maps a value to [0, 12) with 12, 24, … equivalent to 0.
func normDial12(x float64) float64 {
	v := math.Mod(x, 12)
	if v < 0 {
		v += 12
	}
	return v
}

// dialSeparation12 is the shortest distance between two dial positions on a
// uniform 12-hour circle (each unit is one hour).
func dialSeparation12(a, b float64) float64 {
	pa := normDial12(a)
	pb := normDial12(b)
	d := math.Abs(pa - pb)
	if d > 6 {
		d = 12 - d
	}
	return d
}

// nearestUniformDialHour picks the integer hour in [1, 12] closest to h on a
// uniform dial. Ties favor the smaller hour number.
func nearestUniformDialHour(h float64) int {
	bestH := 1
	bestD := math.Inf(1)
	for k := 1; k <= 12; k++ {
		d := dialSeparation12(h, float64(k))
		if d < bestD || (d == bestD && k < bestH) {
			bestD = d
			bestH = k
		}
	}
	return bestH
}
