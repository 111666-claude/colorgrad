package colorgrad

import (
	"math"
	"slices"
)

const (
	// Number of interior probes used to verify a candidate segment before it
	// is accepted. The worst probe also becomes the next split point.
	linSamples = 15

	// Fraction of the requested threshold that refinement targets. The
	// remaining margin absorbs the error that can hide between probes, so the
	// finished gradient stays below the threshold even when sampled at points
	// the refinement never evaluated.
	linSafety = 0.6

	// Hard limits that keep refinement bounded even for pathological curves
	// such as hard edges or 8-bit quantized presets.
	linMaxDepth  = 24
	linMaxStops  = 4096
	linJumpIter  = 48
	linJumpWidth = 1e-9
)

type gradFn func(t float64) Color

type linStop struct {
	t float64
	c Color
}

type linearizer struct {
	grad      gradFn
	target    float64
	minWidth  float64
	jumpWidth float64
	stops     []linStop
}

// linearize converts an arbitrary gradient function into a piecewise-linear
// (in RGBA space) gradient whose error stays below threshold.
//
// The curve is refined per interval with adaptive sampling: a candidate
// segment is accepted only after every probe along it is within target error,
// and otherwise it is split at the worst probe. Because acceptance is checked
// at many points instead of only the midpoint, S-shaped segments, segments
// that span a kink, and segments whose error peaks off-center are all caught.
func linearize(grad gradFn, breaks []float64, dmin, dmax, threshold float64) Gradient {
	threshold = normalizeThreshold(threshold)

	if math.IsNaN(dmin) || math.IsNaN(dmax) || math.IsInf(dmin, 0) || math.IsInf(dmax, 0) || !(dmax > dmin) {
		return newLinearGradient([]Color{grad(dmin).Clamp()}, []float64{dmin}, BlendRgb)
	}

	// Breakpoints are positions where the source curve changes shape (stops,
	// kinks, hard edges). Pinning them as mandatory stops keeps a corner from
	// being smeared across a single straight segment.
	knots := make([]float64, 0, len(breaks)+2)
	knots = append(knots, dmin)
	if len(breaks) > 0 {
		bs := slices.Clone(breaks)
		slices.Sort(bs)
		for _, t := range bs {
			if t > knots[len(knots)-1] && t < dmax {
				knots = append(knots, t)
			}
		}
	}
	knots = append(knots, dmax)

	lz := &linearizer{
		grad:      grad,
		target:    linSafety * threshold,
		minWidth:  (dmax - dmin) * 1e-12,
		jumpWidth: (dmax - dmin) * linJumpWidth,
		stops:     make([]linStop, 0, 64),
	}

	for i := 0; i+1 < len(knots); i++ {
		a, b := knots[i], knots[i+1]
		ca := grad(a).Clamp()
		cb := grad(b).Clamp()
		if i == 0 {
			lz.stops = append(lz.stops, linStop{a, ca})
		}
		lz.refine(a, b, ca, cb, 0)
	}

	positions := make([]float64, len(lz.stops))
	colors := make([]Color, len(lz.stops))
	for i, s := range lz.stops {
		positions[i] = s.t
		colors[i] = s.c
	}

	return newLinearGradient(colors, positions, BlendRgb)
}

func normalizeThreshold(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return 0.007
	}
	return clamp(threshold, 0.001, 0.035)
}

// refine accepts [a, b] as a straight segment when every probe lies within the
// target error, otherwise it splits and recurses.
func (lz *linearizer) refine(a, b float64, ca, cb Color, depth int) {
	width := b - a
	if depth >= linMaxDepth || len(lz.stops) >= linMaxStops || width <= lz.minWidth {
		lz.stops = append(lz.stops, linStop{b, cb})
		return
	}

	worst, frac, cworst := lz.probe(a, b, ca, cb)
	if worst <= lz.target {
		lz.stops = append(lz.stops, linStop{b, cb})
		return
	}

	// A segment that cannot be approximated no matter how it is split is
	// hiding a discontinuity (for example an 8-bit quantized preset, or a
	// sharp gradient). Refining such a segment produces an unbounded number
	// of stops, so isolate the jump between two nearly coincident stops
	// instead of binary-searching it down to the depth limit.
	if jp, jq, cjp, cjq, ok := lz.locateJump(a, b, ca, cb, frac); ok && jp > a && jq < b {
		lz.refine(a, jp, ca, cjp, depth+1)
		lz.stops = append(lz.stops, linStop{jq, cjq})
		lz.refine(jq, b, cjq, cb, depth+1)
		return
	}

	split := a + width*frac
	if !(split > a && split < b) {
		split = a + width*0.5
	}
	if !(split > a && split < b) {
		lz.stops = append(lz.stops, linStop{b, cb})
		return
	}

	lz.refine(a, split, ca, cworst, depth+1)
	lz.refine(split, b, cworst, cb, depth+1)
}

// locateJump bisects the neighbourhood of the worst probe and reports whether
// the curve has a step there. At a discontinuity the probe's own interval
// contains a jump that bisection can make arbitrarily small; on a merely steep
// curve the jump also shrinks, but the bracket stays wide. The two are told
// apart by how much of the bracketed range the jump occupies.
func (lz *linearizer) locateJump(a, b float64, ca, cb Color, frac float64) (jp, jq float64, cjp, cjq Color, ok bool) {
	width := b - a
	// Bracket the worst probe: the step is within one probe spacing of it.
	p := a + width*(frac-1.0/float64(linSamples+1))
	q := a + width*(frac+1.0/float64(linSamples+1))
	if p < a {
		p = a
	}
	if q > b {
		q = b
	}
	if !(q > p) {
		return 0, 0, Color{}, Color{}, false
	}
	cp := lz.grad(p).Clamp()
	cq := lz.grad(q).Clamp()

	jumpCut := lz.target
	origWidth := q - p
	// The step must be located to a small fraction of the original bracket;
	// a steep but continuous curve cannot shrink its span that far.
	maxSpan := origWidth * 0.02
	stopWidth := lz.jumpWidth
	if maxSpan < stopWidth {
		maxSpan = stopWidth
	}

	for iter := 0; iter < linJumpIter; iter++ {
		if q-p <= stopWidth {
			return p, q, cp, cq, true
		}
		m := p + (q-p)*0.5
		if !(m > p && m < q) {
			return p, q, cp, cq, true
		}
		cm := lz.grad(m).Clamp()
		dLeft := colorDiff(cp, cm)
		dRight := colorDiff(cm, cq)

		// Both sides shrink together: steep but continuous.
		if dLeft <= jumpCut && dRight <= jumpCut {
			return 0, 0, Color{}, Color{}, false
		}

		// One side is flat while the other keeps the jump: the step is in the
		// other half.
		if dLeft <= jumpCut {
			p, cp = m, cm
			continue
		}
		if dRight <= jumpCut {
			q, cq = m, cm
			continue
		}

		// Both sides are large: the bracket straddles two nearby steps (as in
		// a quantized preset, where stair treads meet). Let ordinary
		// refinement narrow the bracket to a single step first.
		return 0, 0, Color{}, Color{}, false
	}

	if q-p > maxSpan {
		return 0, 0, Color{}, Color{}, false
	}
	return p, q, cp, cq, true
}

// probe measures the deviation between the source curve and the straight
// segment [a, b] at a fixed set of interior fractions, and reports the worst
// one along with its color.
func (lz *linearizer) probe(a, b float64, ca, cb Color) (worst, frac float64, cworst Color) {
	frac = 0.5
	cworst = cb
	for k := 1; k <= linSamples; k++ {
		f := float64(k) / float64(linSamples+1)
		c := lz.grad(a + (b-a)*f).Clamp()
		if d := colorDiff(c, blendRgb(ca, cb, f)); d > worst {
			worst, frac, cworst = d, f, c
		}
	}
	return
}

// colorDiff is the Euclidean distance between two colors in RGBA space,
// normalized to [0.0, 1.0].
func colorDiff(a, b Color) float64 {
	dr := a.R - b.R
	dg := a.G - b.G
	db := a.B - b.B
	da := a.A - b.A
	return math.Sqrt(dr*dr+dg*dg+db*db+da*da) / 2.0
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
