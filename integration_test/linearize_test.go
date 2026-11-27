package integration_test

import (
	"iter"
	"testing"

	"github.com/mazznoer/colorgrad"
)

func iterCount[K, V any](seq iter.Seq2[K, V]) int {
	n := 0
	for range seq {
		n++
	}
	return n
}

func TestLinearize(t *testing.T) {
	// Simple (2 colors)
	{
		g0, err := colorgrad.NewGradient().
			HtmlColors("#00f", "#f00").
			Mode(colorgrad.BlendRgb).
			Interpolation(colorgrad.InterpolationLinear).
			Build()

		testTrue(t, err == nil)

		g1 := g0.Linearize(0.005)
		lg := g1.Core.(*colorgrad.LinearGradient)

		test(t, lg.Mode(), colorgrad.BlendRgb)
		test(t, iterCount(lg.Stops()), 2)
	}

	// Simple (4 colors)
	{
		g0, err := colorgrad.NewGradient().
			HtmlColors("#00f", "#f00", "#fff", "#ff0").
			Mode(colorgrad.BlendRgb).
			Interpolation(colorgrad.InterpolationLinear).
			Build()

		testTrue(t, err == nil)

		g1 := g0.Linearize(0.005)
		lg := g1.Core.(*colorgrad.LinearGradient)

		test(t, iterCount(lg.Stops()), 4)
	}

	// Complex
	{
		g0, err := colorgrad.NewGradient().
			HtmlColors("#00f", "#f00", "#fff", "#ff0").
			Mode(colorgrad.BlendOklab).
			Interpolation(colorgrad.InterpolationBasis).
			Build()

		testTrue(t, err == nil)

		g1 := g0.Linearize(0.005)
		testTrue(t, g1.Core.(*colorgrad.LinearGradient).Mode() == colorgrad.BlendRgb)

		// The approximation must follow the source curve closely, so sampled
		// colors should stay within one 8-bit step of the original gradient.
		reference := g0.Colors(36)
		approximated := g1.Colors(36)
		for i := range reference {
			x := commonRgba8(reference[i])
			y := commonRgba8(approximated[i])
			for j := range x {
				delta := int(x[j]) - int(y[j])
				if delta < 0 {
					delta = -delta
				}
				if delta > 1 {
					t.Errorf("index %d: %s vs %s (channel %d differs by %d)",
						i, reference[i].HexString(), approximated[i].HexString(), j, delta)
					break
				}
			}
		}
	}
}
