package colorgrad

import (
	"iter"
	"math"
	"slices"
)

type LinearGradient struct {
	colors      [][4]float64
	positions   []float64
	min, max    float64
	mode        BlendMode
	first, last Color
	hints       []float64
}

func (lg *LinearGradient) At(t float64) Color {
	if math.IsNaN(t) {
		return Color{A: 1}
	}

	if t <= lg.min {
		return lg.first
	}

	if t >= lg.max {
		return lg.last
	}

	lo, hi := 0, len(lg.positions)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if lg.positions[mid] < t {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	i := lo - 1
	p1 := lg.positions[i]
	p2 := lg.positions[lo]
	span := p2 - p1

	u := 0.0
	if span > 0 {
		u = (t - p1) / span
	}
	if !cssFinite(u) {
		u = 0
	}

	// Apply the CSS transition hint easing for this segment, if any.
	if i < len(lg.hints) {
		if hp := lg.hints[i]; !math.IsNaN(hp) && span > 0 {
			h := (hp - p1) / span
			u = cssHintWeight(h, u)
		}
	}

	a, b, c, d := linearInterpolate(lg.colors[i], lg.colors[lo], u)

	switch lg.mode {
	case BlendLinearRgb:
		return LinearRgb(a, b, c, d)
	case BlendLab:
		return Lab(a, b, c, d).Clamp()
	case BlendOklab:
		return Oklab(a, b, c, d).Clamp()
	default:
		return Color{R: a, G: b, B: c, A: d}
	}
}

func (lg *LinearGradient) Mode() BlendMode {
	return lg.mode
}

func (lg *LinearGradient) Stops() iter.Seq2[float64, [4]float64] {
	return func(yield func(float64, [4]float64) bool) {
		for i, t := range lg.positions {
			if !yield(t, lg.colors[i]) {
				return
			}
		}
	}
}

func newLinearGradient(colors []Color, positions []float64, mode BlendMode) Gradient {
	pos := slices.Clone(positions)

	g := &LinearGradient{
		colors:    convertColors(colors, mode),
		positions: pos,
		min:       pos[0],
		max:       pos[len(pos)-1],
		mode:      mode,
		first:     colors[0],
		last:      colors[len(pos)-1],
	}

	return Gradient{
		Core: g,
		Min:  g.min,
		Max:  g.max,
	}
}

// newLinearGradientWithHints builds a linear gradient that additionally eases
// each segment according to hints. hints must have length len(positions)-1;
// math.NaN() entries mark segments without a hint.
func newLinearGradientWithHints(colors []Color, positions, hints []float64, mode BlendMode) Gradient {
	grad := newLinearGradient(colors, positions, mode)
	g := grad.Core.(*LinearGradient)

	g.hints = make([]float64, len(g.positions)-1)
	for i := range g.hints {
		g.hints[i] = math.NaN()
		if i < len(hints) {
			g.hints[i] = hints[i]
		}
	}

	return grad
}
