package colorgrad

import (
	"io"
	"math"
	"strings"
	"testing"
)

func Test_GIMPGradient(t *testing.T) {
	black := Rgb(0, 0, 0, 1)
	red := Rgb(1, 0, 0, 1)
	blue := Rgb(0, 0, 1, 1)

	// Black to white
	ggr := "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 0 0 0 1 1 1 1 1 0 0 0 0"
	grad, name, err := ParseGgr(strings.NewReader(ggr), black, black)
	test(t, err, nil)
	test(t, name, "My Gradient")
	test(t, grad.At(0).HexString(), "#000000")
	test(t, grad.At(1).HexString(), "#ffffff")
	test(t, grad.At(-0.5).HexString(), "#000000")
	test(t, grad.At(1.5).HexString(), "#ffffff")
	test(t, grad.At(math.NaN()).HexString(), "#000000")

	// Foreground to background
	ggr = "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 0 0 0 1 1 1 1 1 0 0 1 3"
	grad, name, err = ParseGgr(strings.NewReader(ggr), red, blue)
	test(t, err, nil)
	test(t, name, "My Gradient")
	test(t, grad.At(0).HexString(), "#ff0000")
	test(t, grad.At(1).HexString(), "#0000ff")

	// Blending function: step
	ggr = "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 0 0 1 0 0 1 1 5 0 0 0"
	grad, name, err = ParseGgr(strings.NewReader(ggr), black, black)
	test(t, err, nil)
	test(t, name, "My Gradient")
	test(t, grad.At(0.00).HexString(), "#ff0000")
	test(t, grad.At(0.25).HexString(), "#ff0000")
	test(t, grad.At(0.49).HexString(), "#ff0000")
	test(t, grad.At(0.51).HexString(), "#0000ff")
	test(t, grad.At(0.75).HexString(), "#0000ff")
	test(t, grad.At(1.00).HexString(), "#0000ff")

	// Coloring type: HSV CCW (white to blue)
	ggr = "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 1 1 1 0 0 1 1 0 1 0 0"
	grad, name, err = ParseGgr(strings.NewReader(ggr), black, black)
	test(t, err, nil)
	test(t, name, "My Gradient")
	test(t, grad.At(0.0).HexString(), "#ffffff")
	test(t, grad.At(0.5).HexString(), "#80ff80")
	test(t, grad.At(1.0).HexString(), "#0000ff")

	// Coloring type: HSV CW (white to blue)
	ggr = "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 1 1 1 0 0 1 1 0 2 0 0"
	grad, name, err = ParseGgr(strings.NewReader(ggr), black, black)
	test(t, err, nil)
	test(t, name, "My Gradient")
	test(t, grad.At(0.0).HexString(), "#ffffff")
	test(t, grad.At(0.5).HexString(), "#ff80ff")
	test(t, grad.At(1.0).HexString(), "#0000ff")

	// Invalid formats

	data := []string{
		"",
		" ",
		"GIMP Palette\nName: Gold\n#\n252 252 128",
		"GIMP Gradient\nxx",
		"GIMP Gradient\nName: Gradient\nx",
		"GIMP Gradient\nName: Gradient\n1\n0 0 0",
	}
	for _, s := range data {
		_, _, err := ParseGgr(strings.NewReader(s), black, black)
		testTrue(t, err != nil)
	}
}

func Test_GGWriteRoundTrip(t *testing.T) {
	red := Rgb(1, 0, 0, 1)
	blue := Rgb(0, 0, 1, 1)
	green := Rgb(0, 1, 0, 1)
	yellow := Rgb(1, 1, 0, 1)

	// Three segments covering all endpoint color types, HSV coloring and step.
	ggr := "GIMP Gradient\n" +
		"Name: Round Trip\n" +
		"3\n" +
		"0 0.2 0.4 1 0 0 1 0 0 1 1 0 0 1 3\n" +
		"0.4 0.6 0.7 1 1 1 0.5 0 0.5 0 0.25 0 1 2 4\n" +
		"0.7 0.8 1 0.2 0.4 0.6 0.8 0 0 0 1 5 2 0 0"

	grad, name, err := ParseGgr(strings.NewReader(ggr), red, blue)
	test(t, err, nil)
	test(t, name, "Round Trip")

	var buf strings.Builder
	err = WriteGgr(&buf, grad)
	test(t, err, nil)

	// The written file must be a valid .ggr with the exact structure.
	want := "GIMP Gradient\n" +
		"Name: Round Trip\n" +
		"3\n" +
		"0 0.2 0.4 1 0 0 1 0 0 1 1 0 0 1 3\n" +
		"0.4 0.6 0.7 1 1 1 0.5 0 0.5 0 0.25 0 1 2 4\n" +
		"0.7 0.8 1 0.2 0.4 0.6 0.8 0 0 0 1 5 2 0 0\n"
	test(t, buf.String(), want)

	// Re-parse with the original foreground/background: identical gradient.
	grad2, name2, err := ParseGgr(strings.NewReader(buf.String()), red, blue)
	test(t, err, nil)
	test(t, name2, "Round Trip")
	for _, p := range []float64{0, 0.2, 0.4, 0.6, 0.7, 0.8, 1} {
		test(t, grad2.At(p).HexString(), grad.At(p).HexString())
	}

	// Re-parse with a different foreground/background: dynamic endpoints move,
	// fixed endpoints stay unchanged.
	grad3, _, err := ParseGgr(strings.NewReader(buf.String()), green, yellow)
	test(t, err, nil)

	// Segment 0 left endpoint (t == 0) is foreground.
	test(t, grad3.At(0).HexString(), green.HexString())
	// A point inside segment 0 (its midpoint) blends foreground halfway
	// toward the background endpoint.
	c := grad3.At(0.2)
	test(t, c.R, 0.5)
	test(t, c.G, 1.0)
	test(t, c.B, 0.0)
	// Boundary 0.4 belongs to segment 1, whose left endpoint is
	// foreground transparent (alpha 0, RGB taken from foreground).
	c = grad3.At(0.4)
	test(t, c.A, 0.0)
	test(t, c.R, green.R)
	test(t, c.G, green.G)
	test(t, c.B, green.B)
	// Segment 1 right endpoint (t == 0.7) is background transparent.
	c = grad3.At(0.7)
	test(t, c.A, 0.0)
	test(t, c.R, yellow.R)
	test(t, c.G, yellow.G)
	test(t, c.B, yellow.B)
	// Segment 2 endpoints are fixed colors and must not move. Segment 2 uses
	// step blending, so sample past its midpoint to reach its right endpoint.
	test(t, grad3.At(1).HexString(), grad.At(1).HexString())
	seg2Right := grad.Core.(GimpGradient).segments[2].rcolor
	test(t, grad3.At(0.9).HexString(), seg2Right.HexString())

	// WithColors rebinds the dynamic endpoints of the in-memory gradient too.
	grad4 := grad.Core.(GimpGradient).WithColors(green, yellow)
	test(t, grad4.Name(), "Round Trip")
	test(t, grad4.At(0).HexString(), green.HexString())
	c = grad4.At(0.2)
	test(t, c.R, 0.5)
	test(t, c.G, 1.0)
	test(t, c.B, 0.0)
}

func Test_GGWriteFixedSamples(t *testing.T) {
	black := Rgb(0, 0, 0, 1)

	// FG -> BG gradient, identical data to the existing parse test.
	fgBg := "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 0 0 0 1 1 1 1 1 0 0 1 3"
	red := Rgb(1, 0, 0, 1)
	blue := Rgb(0, 0, 1, 1)
	grad, name, err := ParseGgr(strings.NewReader(fgBg), red, blue)
	test(t, err, nil)
	test(t, name, "My Gradient")

	var buf strings.Builder
	err = WriteGgr(&buf, grad)
	test(t, err, nil)
	test(t, buf.String(), "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 0 0 0 1 1 1 1 1 0 0 1 3\n")

	// Endpoint types survive the round trip.
	grad2, _, err := ParseGgr(strings.NewReader(buf.String()), red, blue)
	test(t, err, nil)
	test(t, grad2.At(0).HexString(), "#ff0000")
	test(t, grad2.At(1).HexString(), "#0000ff")

	// Step blending fixed sample.
	stepGgr := "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 0 0 1 0 0 1 1 5 0 0 0"
	stepGrad, _, err := ParseGgr(strings.NewReader(stepGgr), black, black)
	test(t, err, nil)
	buf.Reset()
	err = WriteGgr(&buf, stepGrad)
	test(t, err, nil)
	test(t, buf.String(), "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 0 0 1 0 0 1 1 5 0 0 0\n")
	stepGrad2, _, err := ParseGgr(strings.NewReader(buf.String()), black, black)
	test(t, err, nil)
	test(t, stepGrad2.At(0.00).HexString(), "#ff0000")
	test(t, stepGrad2.At(0.49).HexString(), "#ff0000")
	test(t, stepGrad2.At(0.51).HexString(), "#0000ff")
	test(t, stepGrad2.At(1.00).HexString(), "#0000ff")

	// HSV CCW fixed sample, key positions must match after the round trip.
	hsvGgr := "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 1 1 1 0 0 1 1 0 1 0 0"
	hsvGrad, _, err := ParseGgr(strings.NewReader(hsvGgr), black, black)
	test(t, err, nil)
	buf.Reset()
	err = WriteGgr(&buf, hsvGrad)
	test(t, err, nil)
	test(t, buf.String(), "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 1 1 1 0 0 1 1 0 1 0 0\n")
	hsvGrad2, _, err := ParseGgr(strings.NewReader(buf.String()), black, black)
	test(t, err, nil)
	test(t, hsvGrad2.At(0.0).HexString(), "#ffffff")
	test(t, hsvGrad2.At(0.5).HexString(), "#80ff80")
	test(t, hsvGrad2.At(1.0).HexString(), "#0000ff")

	// HSV CW fixed sample.
	hsvCwGgr := "GIMP Gradient\nName: My Gradient\n1\n0 0.5 1 1 1 1 1 0 0 1 1 0 2 0 0"
	hsvCwGrad, _, err := ParseGgr(strings.NewReader(hsvCwGgr), black, black)
	test(t, err, nil)
	buf.Reset()
	err = WriteGgr(&buf, hsvCwGrad)
	test(t, err, nil)
	hsvCwGrad2, _, err := ParseGgr(strings.NewReader(buf.String()), black, black)
	test(t, err, nil)
	test(t, hsvCwGrad2.At(0.5).HexString(), "#ff80ff")
	test(t, hsvCwGrad2.At(1.0).HexString(), "#0000ff")
}

// failingWriter always returns an error to surface write failures.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func Test_GGWriteErrors(t *testing.T) {
	red := Rgb(1, 0, 0, 1)
	blue := Rgb(0, 0, 1, 1)

	validGgr := "GIMP Gradient\nName: x\n1\n0 0.5 1 0 0 0 1 1 1 1 1 0 0 1 3"
	validGrad, _, err := ParseGgr(strings.NewReader(validGgr), red, blue)
	test(t, err, nil)

	// Non-GIMP gradient cannot be written.
	nonGgr, berr := NewGradient().Build()
	test(t, berr, nil)
	err = WriteGgr(&strings.Builder{}, nonGgr)
	testTrue(t, err != nil)

	// Write failure must be reported.
	err = WriteGgr(failingWriter{}, validGrad)
	testTrue(t, err != nil)

	base := func(segs ...gimpSegment) Gradient {
		return Gradient{
			Core: GimpGradient{segments: segs, min: 0, max: 1, fg: red, bg: blue, name: "x"},
			Min:  0,
			Max:  1,
		}
	}
	fixedSeg := func(l, m, r float64) gimpSegment {
		return gimpSegment{
			lcolor:   Rgb(0, 0, 0, 1),
			rcolor:   Rgb(1, 1, 1, 1),
			lctype:   colorFixed,
			rctype:   colorFixed,
			lpos:     l,
			mpos:     m,
			rpos:     r,
			blending: linear,
			coloring: rgb,
		}
	}

	bad := []Gradient{
		// Coordinates out of order: middle > right.
		base(fixedSeg(0, 0.8, 0.5)),
		// Coordinates out of order: left > middle.
		base(fixedSeg(0.6, 0.4, 1)),
		// First segment does not start at 0.
		base(fixedSeg(0.1, 0.5, 1)),
		// Last segment does not end at 1.
		base(fixedSeg(0, 0.5, 0.9)),
		// Non-contiguous segments.
		base(fixedSeg(0, 0.2, 0.4), fixedSeg(0.5, 0.7, 1)),
		// Non-finite coordinate.
		base(fixedSeg(0, math.NaN(), 1)),
		base(fixedSeg(0, math.Inf(1), 1)),
		// Non-finite fixed endpoint color.
		func() Gradient {
			g := base(fixedSeg(0, 0.5, 1))
			seg := g.Core.(GimpGradient).segments
			seg[0].lcolor = Rgb(math.NaN(), 0, 0, 1)
			return g
		}(),
	}

	for i, g := range bad {
		err := WriteGgr(&strings.Builder{}, g)
		if err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}
