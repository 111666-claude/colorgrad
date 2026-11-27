package colorgrad

import (
	"fmt"
	"math"
	"testing"
)

// linearizeRGBAError samples the source gradient and the linearized gradient
// densely and returns the largest Euclidean RGBA distance found.
//
// A non-uniform sampling offset is applied so the check does not accidentally
// line up with the probe grid used internally by Linearize.
func linearizeRGBAError(src, lin Gradient, count int, offset float64) float64 {
	dmin, dmax := src.Domain()
	worst := 0.0
	for i := 0; i <= count; i++ {
		t := dmin + (dmax-dmin)*(float64(i)+offset)/float64(count+1)
		if t < dmin || t > dmax {
			continue
		}
		if d := colorDiff(src.At(t).Clamp(), lin.At(t).Clamp()); d > worst {
			worst = d
		}
	}
	return worst
}

func checkLinearizeBound(t *testing.T, name string, src Gradient, thresholds []float64) {
	t.Helper()
	prev := math.Inf(1)
	for _, threshold := range thresholds {
		lin := src.Linearize(threshold)

		worst := 0.0
		for _, offset := range []float64{0, 0.13, 0.37, 0.61, 0.89} {
			if e := linearizeRGBAError(src, lin, 4000, offset); e > worst {
				worst = e
			}
		}
		if e := linearizeRGBAError(src, lin, 50000, 0.37); e > worst {
			worst = e
		}

		// A stricter threshold must not produce a larger measured error.
		if worst > prev+1e-12 {
			t.Errorf("%s: threshold %v gave error %v, larger than %v for the previous threshold",
				name, threshold, worst, prev)
		}
		prev = worst

		// The measured error must be within the requested threshold.
		if worst > threshold {
			t.Errorf("%s: threshold %v exceeded, measured error %v", name, threshold, worst)
		}
	}
}

func Test_Linearize_Accuracy(t *testing.T) {
	builder := func() *GradientBuilder {
		return NewGradient().
			HtmlColors("#00f", "#f00", "#fff", "#ff0").
			Mode(BlendRgb)
	}
	thresholds := []float64{0.035, 0.02, 0.01, 0.007, 0.005, 0.004, 0.003, 0.0025, 0.002, 0.0015, 0.001}

	for _, interp := range []Interpolation{
		InterpolationLinear,
		InterpolationSmoothstep,
		InterpolationBasis,
		InterpolationCatmullRom,
	} {
		g, err := builder().Interpolation(interp).Build()
		if err != nil {
			t.Fatal(err)
		}
		checkLinearizeBound(t, interp.String(), g, thresholds)
	}

	for _, mode := range []BlendMode{
		BlendRgb,
		BlendLinearRgb,
		BlendLab,
		BlendOklab,
	} {
		g, err := builder().Mode(mode).Interpolation(InterpolationCatmullRom).Build()
		if err != nil {
			t.Fatal(err)
		}
		checkLinearizeBound(t, "CatmullRom/"+mode.String(), g, thresholds)
	}

	// Custom domain, where stop positions are not fractions of the domain.
	g, err := NewGradient().
		HtmlColors("#2ec149", "#efd037", "#ed6212", "#db1ed5").
		Domain(20, 25, 37, 59).
		Mode(BlendRgb).
		Interpolation(InterpolationCatmullRom).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	checkLinearizeBound(t, "CatmullRom/custom-domain", g, thresholds)
}

// Regression test for a four-stop Catmull-Rom gradient at threshold 0.003.
// The previous implementation measured about 0.0108 RGBA error here because
// it only checked the midpoint of each candidate interval and then dropped
// points based on a chord that had already drifted away from the curve.
func Test_Linearize_CatmullRom_FourStops(t *testing.T) {
	g, err := NewGradient().
		HtmlColors("#00f", "#f00", "#fff", "#ff0").
		Mode(BlendRgb).
		Interpolation(InterpolationCatmullRom).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	lin := g.Linearize(0.003)

	for _, offset := range []float64{0, 0.37, 0.89} {
		err := linearizeRGBAError(g, lin, 20000, offset)
		if err > 0.003 {
			t.Fatalf("offset %v: error %v exceeds threshold 0.003", offset, err)
		}
	}
}

func Test_Linearize_StopInvariants(t *testing.T) {
	build := func(colors []string, positions []float64) Gradient {
		gb := NewGradient().
			HtmlColors(colors...).
			Mode(BlendOklab).
			Interpolation(InterpolationCatmullRom)
		if len(positions) > 0 {
			gb = gb.Domain(positions...)
		}
		g, err := gb.Build()
		if err != nil {
			t.Fatal(err)
		}
		return g
	}

	inputs := []Gradient{
		build([]string{"#00f", "#f00", "#fff", "#ff0"}, nil),
		build([]string{"#2ec149", "#efd037", "#ed6212", "#db1ed5"}, []float64{20, 25, 37, 59}),
		Turbo(),
		Sinebow(),
	}

	for _, g := range inputs {
		for _, threshold := range []float64{0.035, 0.01, 0.003, 0.001} {
			lin := g.Linearize(threshold)
			lg := lin.Core.(*LinearGradient)

			dmin, dmax := g.Domain()
			lmin, lmax := lin.Domain()
			if lmin != dmin || lmax != dmax {
				t.Fatalf("domain changed: got [%v, %v], want [%v, %v]", lmin, lmax, dmin, dmax)
			}

			positions := make([]float64, 0, 4)
			colors := make([]Color, 0, 4)
			for pos, col := range lg.Stops() {
				positions = append(positions, pos)
				colors = append(colors, Rgb(col[0], col[1], col[2], col[3]))
			}
			if len(positions) < 2 {
				t.Fatalf("only %d stops", len(positions))
			}
			for i := 1; i < len(positions); i++ {
				if !(positions[i] > positions[i-1]) {
					t.Fatalf("stops are not ordered at index %d: %v", i, positions)
				}
			}
			for i, pos := range positions {
				c := colors[i]
				if c.R < 0 || c.R > 1 || c.G < 0 || c.G > 1 || c.B < 0 || c.B > 1 || c.A < 0 || c.A > 1 {
					t.Fatalf("invalid color %v at stop %v", c, pos)
				}
				if want := g.At(pos).Clamp(); colorDiff(c, want) > 1e-9 {
					t.Fatalf("stop color at %v is %v, source gives %v", pos, c, want)
				}
			}

			// Endpoints and outside the domain must match the source.
			for _, pos := range []float64{dmin, dmax, dmin - 1, dmax + 1} {
				if colorDiff(lin.At(pos).Clamp(), g.At(pos).Clamp()) > 1e-9 {
					t.Fatalf("color at %v differs from source", pos)
				}
			}
		}
	}
}

func Test_Linearize_Deterministic(t *testing.T) {
	g, err := NewGradient().
		HtmlColors("#00f", "#f00", "#fff", "#ff0").
		Mode(BlendOklab).
		Interpolation(InterpolationCatmullRom).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	reference := g.Linearize(0.003)
	refPositions := make([]float64, 0, 8)
	refPositions = append(refPositions, reference.Core.(*LinearGradient).positions...)

	for i := 0; i < 5; i++ {
		again := g.Linearize(0.003)
		positions := again.Core.(*LinearGradient).positions
		if len(positions) != len(refPositions) {
			t.Fatalf("build %d produced %d stops, want %d", i, len(positions), len(refPositions))
		}
		for j := range positions {
			if positions[j] != refPositions[j] {
				t.Fatalf("build %d differs at stop %d: %v != %v", i, j, positions[j], refPositions[j])
			}
		}
	}
}

func Test_Linearize_BoundedStopCount(t *testing.T) {
	// 8-bit quantized presets are full of tiny steps. They must not make stop
	// counts explode when the threshold is small.
	for _, g := range []Gradient{Turbo(), Cividis()} {
		for _, threshold := range []float64{0.01, 0.005, 0.003, 0.001} {
			lin := g.Linearize(threshold)
			if n := len(lin.Core.(*LinearGradient).positions); n > linMaxStops {
				t.Fatalf("threshold %v produced %d stops, limit is %d", threshold, n, linMaxStops)
			}
		}
	}
}

func Test_Linearize_ThresholdNormalization(t *testing.T) {
	g, err := NewGradient().
		HtmlColors("#00f", "#f00", "#fff", "#ff0").
		Interpolation(InterpolationCatmullRom).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Out of range and non-finite thresholds are clamped or defaulted instead
	// of failing.
	for _, threshold := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 0, 2} {
		lin := g.Linearize(threshold)
		if lin.Core.(*LinearGradient) == nil {
			t.Fatalf("threshold %v produced no gradient", threshold)
		}
		if n := len(lin.Core.(*LinearGradient).positions); n < 2 {
			t.Fatalf("threshold %v produced %d stops", threshold, n)
		}
	}
}

func Test_Linearize_DegenerateDomain(t *testing.T) {
	g := Gradient{
		Core: gradFunc(func(t float64) Color {
			return Rgb(t, 0, 0, 1)
		}),
		Min: 1,
		Max: 1,
	}
	for _, threshold := range []float64{0.01, 0, math.NaN()} {
		lin := g.Linearize(threshold)
		if lin.Min != 1 || lin.Max != 1 {
			t.Fatalf("domain changed to [%v, %v]", lin.Min, lin.Max)
		}
		if n := len(lin.Core.(*LinearGradient).positions); n < 1 {
			t.Fatalf("no stops")
		}
	}
}

type gradFunc func(t float64) Color

func (f gradFunc) At(t float64) Color { return f(t) }

func BenchmarkLinearize(b *testing.B) {
	g, err := NewGradient().
		HtmlColors(colors...).
		Mode(BlendOklab).
		Interpolation(InterpolationCatmullRom).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	for _, threshold := range []float64{0.01, 0.003} {
		b.Run(fmt.Sprintf("threshold_%v", threshold), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				g.Linearize(threshold)
			}
		})
	}
}
