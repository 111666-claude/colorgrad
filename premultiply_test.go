package colorgrad

import (
	"math"
	"testing"
)

func testColor(t *testing.T, got, want Color, eps float64) {
	t.Helper()
	if math.Abs(got.R-want.R) > eps ||
		math.Abs(got.G-want.G) > eps ||
		math.Abs(got.B-want.B) > eps ||
		math.Abs(got.A-want.A) > eps {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func Test_PremultipliedMidpoint(t *testing.T) {
	transparentRed := Rgb(1, 0, 0, 0)
	opaqueBlue := Rgb(0, 0, 1, 1)

	interpolations := []Interpolation{
		InterpolationLinear,
		InterpolationSmoothstep,
		InterpolationCatmullRom,
		InterpolationBasis,
	}

	// Default (non-premultiplied) behavior is unchanged: half red, half blue.
	for _, interp := range interpolations {
		grad, err := NewGradient().
			Colors(transparentRed, opaqueBlue).
			Interpolation(interp).
			Build()
		test(t, err, nil)
		testColor(t, grad.At(0.5), Rgb(0.5, 0, 0.5, 0.5), 1e-9)
	}

	// Premultiplied: the midpoint is blue at alpha 0.5 in every color space
	// and every interpolation mode.
	modes := []BlendMode{BlendRgb, BlendLinearRgb, BlendLab, BlendOklab}
	for _, mode := range modes {
		eps := 1e-9
		if mode == BlendLab || mode == BlendOklab {
			// Lab/Oklab matrix round-trips are not exact.
			eps = 5e-3
		}
		for _, interp := range interpolations {
			grad, err := NewGradient().
				Colors(transparentRed, opaqueBlue).
				Mode(mode).
				Interpolation(interp).
				Premultiplied(true).
				Build()
			test(t, err, nil)
			testColor(t, grad.At(0.5), Rgb(0, 0, 1, 0.5), eps)

			// Endpoints are returned as-is.
			testColor(t, grad.At(0), transparentRed, 1e-9)
			testColor(t, grad.At(1), opaqueBlue, 1e-9)
		}
	}
}

func Test_PremultipliedNonZeroAlpha(t *testing.T) {
	// Both endpoints have non-zero alpha.
	c0 := Rgb(1, 0, 0, 0.5)
	c1 := Rgb(0, 0, 1, 1)

	grad, err := NewGradient().
		Colors(c0, c1).
		Premultiplied(true).
		Build()
	test(t, err, nil)

	// Premultiplied midpoint: (0.25, 0, 0.5, 0.75) -> (1/3, 0, 2/3, 0.75)
	testColor(t, grad.At(0.5), Rgb(1.0/3.0, 0, 2.0/3.0, 0.75), 1e-9)

	// Default interpolation mixes the color channels directly.
	grad2, err := NewGradient().
		Colors(c0, c1).
		Build()
	test(t, err, nil)
	testColor(t, grad2.At(0.5), Rgb(0.5, 0, 0.5, 0.75), 1e-9)

	// Fully opaque endpoints: premultiplied and default results are identical.
	grad3, err := NewGradient().
		HtmlColors("red", "lime", "blue").
		Premultiplied(true).
		Build()
	test(t, err, nil)
	grad4, err := NewGradient().
		HtmlColors("red", "lime", "blue").
		Build()
	test(t, err, nil)
	for _, pos := range []float64{0, 0.13, 0.5, 0.77, 1} {
		testColor(t, grad3.At(pos), grad4.At(pos), 1e-9)
	}
}

func Test_PremultipliedZeroAlpha(t *testing.T) {
	// Both endpoints fully transparent: deterministic transparent black.
	grad, err := NewGradient().
		Colors(Rgb(1, 0, 0, 0), Rgb(0, 0, 1, 0)).
		Premultiplied(true).
		Build()
	test(t, err, nil)
	testColor(t, grad.At(0.5), Color{}, 0)
	test(t, grad.At(0.5).HexString(), "#00000000")

	// Transparent color to transparent black.
	grad, err = NewGradient().
		Colors(Rgb(1, 0, 0, 0), Rgb(0, 0, 0, 0)).
		Premultiplied(true).
		Build()
	test(t, err, nil)
	testColor(t, grad.At(0.5), Color{}, 0)
}

func Test_PremultipliedConsistency(t *testing.T) {
	// At, Colors and ColorsIter give consistent results.
	grad, err := NewGradient().
		Colors(Rgb(1, 0, 0, 0), Rgb(0, 1, 0, 0.5), Rgb(0, 0, 1, 1)).
		Domain(-1, 1).
		Mode(BlendOklab).
		Premultiplied(true).
		Build()
	test(t, err, nil)

	dmin, dmax := grad.Domain()
	test(t, dmin, -1.0)
	test(t, dmax, 1.0)

	const n = 11
	colors := grad.Colors(n)
	test(t, len(colors), n)

	i := 0
	for c := range grad.ColorsIter(n) {
		testColor(t, c, colors[i], 0)
		// Colors/ColorsIter sample the same positions as At.
		pos := dmin + float64(i)*(dmax-dmin)/float64(n-1)
		testColor(t, c, grad.At(pos).Clamp(), 1e-9)
		i++
	}
	test(t, i, n)
}

func Test_PremultipliedReset(t *testing.T) {
	gb := NewGradient().
		Colors(Rgb(1, 0, 0, 0), Rgb(0, 0, 1, 1)).
		Premultiplied(true)

	grad, err := gb.Build()
	test(t, err, nil)
	testColor(t, grad.At(0.5), Rgb(0, 0, 1, 0.5), 1e-9)

	// Reset restores the default (non-premultiplied) behavior.
	grad, err = gb.Reset().
		Colors(Rgb(1, 0, 0, 0), Rgb(0, 0, 1, 1)).
		Build()
	test(t, err, nil)
	testColor(t, grad.At(0.5), Rgb(0.5, 0, 0.5, 0.5), 1e-9)
}
