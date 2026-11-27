package colorgrad

import "math"

// hintPlan carries the transition hint (CSS interpolation hint) information for
// a CSS gradient. Colors/positions contain only real color stops (standalone
// hint tokens are excluded) and hints holds one absolute hint position per
// segment between consecutive color stops. A math.NaN() entry means the segment
// has no hint and interpolates linearly.
type hintPlan struct {
	colors    []Color
	positions []float64
	hints     []float64
}

// buildHintPlan collects the real color stops and per-segment transition hints
// from the fixed-up CSS stops. hintIdx contains the indices of standalone hint
// stops inside stops.
func buildHintPlan(stops []cssGradientStop, hintIdx []int) hintPlan {
	isHint := make([]bool, len(stops))
	for _, idx := range hintIdx {
		if idx >= 0 && idx < len(isHint) {
			isHint[idx] = true
		}
	}

	plan := hintPlan{}
	pendingHint := math.NaN()

	for i, stop := range stops {
		if isHint[i] {
			if stop.pos != nil && cssFinite(*stop.pos) {
				pendingHint = *stop.pos
			}
			continue
		}

		if stop.color == nil || stop.pos == nil {
			continue
		}

		if len(plan.positions) > 0 {
			// Hint between the previous real stop and this one.
			plan.hints = append(plan.hints, pendingHint)
		}

		plan.colors = append(plan.colors, *stop.color)
		plan.positions = append(plan.positions, *stop.pos)
		pendingHint = math.NaN()
	}

	return plan
}

// cssHintWeight returns the color weighting at relative sample position x for a
// transition hint located at relative position h inside a segment.
//
// Both h and x are in [0, 1], where 0 is the first color stop and 1 the second.
// Per CSS Images specification the weighting is:
//
//	C = x ^ log_h(0.5) = x ^ (ln(0.5) / ln(h))
//
// so that C(h) == 0.5 (the hint marks the equal-color point). All endpoint and
// degenerate cases are handled with their limiting values so the result is
// always finite and within [0, 1].
func cssHintWeight(h, x float64) float64 {
	if !cssFinite(x) {
		return 0
	}
	x = clamp01(x)

	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}

	if !cssFinite(h) {
		return x
	}
	h = clamp01(h)

	// Limiting behavior for hints sitting on a segment endpoint.
	if h <= 0 {
		return 1
	}
	if h >= 1 {
		return 0
	}

	// A centered hint is equivalent to ordinary linear interpolation.
	if h == 0.5 {
		return x
	}

	// The hint point itself must be an exact equal-color (50%) mix.
	if x == h {
		return 0.5
	}

	w := math.Pow(x, math.Log(0.5)/math.Log(h))
	if !cssFinite(w) {
		return x
	}
	return clamp01(w)
}
