package colorgrad

import (
	"math"
)

type LinearGradient struct {
	colors        [][4]float64
	positions     []float64
	min           float64
	max           float64
	mode          BlendMode
	premultiplied bool
	first         Color
	last          Color
}

func (lg LinearGradient) At(t float64) Color {
	if t <= lg.min {
		return lg.first
	}

	if t >= lg.max {
		return lg.last
	}

	if math.IsNaN(t) {
		return Color{A: 1}
	}

	low := 0
	high := len(lg.positions)

	for low < high {
		mid := (low + high) / 2
		if lg.positions[mid] < t {
			low = mid + 1
		} else {
			high = mid
		}
	}

	if low == 0 {
		low = 1
	}

	p1 := lg.positions[low-1]
	p2 := lg.positions[low]
	t = (t - p1) / (p2 - p1)
	a, b, c, d := linearInterpolate(lg.colors[low-1], lg.colors[low], t)

	return toColor(a, b, c, d, lg.mode, lg.premultiplied)
}

func (lg LinearGradient) Mode() BlendMode {
	return lg.mode
}

func (lg LinearGradient) Colors() [][4]float64 {
	return lg.colors
}

func (lg LinearGradient) Positions() []float64 {
	return lg.positions
}

func newLinearGradient(colors []Color, positions []float64, mode BlendMode, premultiplied bool) Gradient {
	gradbase := LinearGradient{
		colors:        convertColors(colors, mode, premultiplied),
		positions:     positions,
		min:           positions[0],
		max:           positions[len(positions)-1],
		mode:          mode,
		premultiplied: premultiplied,
		first:         colors[0],
		last:          colors[len(colors)-1],
	}

	return Gradient{
		Core: gradbase,
		Min:  positions[0],
		Max:  positions[len(positions)-1],
	}
}
