package colorgrad

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strings"
)

// References:
// https://gitlab.gnome.org/GNOME/gimp/-/blob/master/devel-docs/ggr.txt
// https://gitlab.gnome.org/GNOME/gimp/-/blob/master/app/core/gimpgradient.c
// https://gitlab.gnome.org/GNOME/gimp/-/blob/master/app/core/gimpgradient-load.c

const epsilon = 1e-10
const fracPi2 = math.Pi / 2

type blendingType int

const (
	linear blendingType = iota
	curved
	sinusoidal
	sphericalIncreasing
	sphericalDecreasing
	step
)

type coloringType int

const (
	rgb coloringType = iota
	hsvCcw
	hsvCw
)

// Endpoint color type, mirrors GIMP's GimpGradientColor enum.
type endpointColorType int

const (
	colorFixed                 endpointColorType = 0
	colorForeground            endpointColorType = 1
	colorForegroundTransparent endpointColorType = 2
	colorBackground            endpointColorType = 3
	colorBackgroundTransparent endpointColorType = 4
)

type gimpSegment struct {
	// Left endpoint color (the raw color value; only meaningful for fixed endpoints)
	lcolor Color
	// Right endpoint color (the raw color value; only meaningful for fixed endpoints)
	rcolor Color
	// Left endpoint color type
	lctype endpointColorType
	// Right endpoint color type
	rctype endpointColorType
	// Left endpoint coordinate
	lpos float64
	// Midpoint coordinate
	mpos float64
	// Right endpoint coordinate
	rpos float64
	// Blending function type
	blending blendingType
	// Coloring type
	coloring coloringType
}

// GimpGradient is a multi-segment gradient in the GIMP gradient (.ggr) model.
//
// Endpoints may reference the foreground/background colors instead of fixed
// color values. The referenced colors are stored on the gradient itself and
// can be rebound with [GimpGradient.WithColors], so a parsed gradient stays
// editable after parsing.
type GimpGradient struct {
	segments []gimpSegment
	min      float64
	max      float64
	// Foreground color used by foreground-type endpoints
	fg Color
	// Background color used by background-type endpoints
	bg   Color
	name string
}

// Name returns the gradient name stored in the .ggr file header.
func (ggr GimpGradient) Name() string {
	return ggr.name
}

// WithColors returns a copy of the gradient with the foreground and background
// colors used by foreground/background-type endpoints replaced. Fixed-color
// endpoints are not affected.
func (ggr GimpGradient) WithColors(fg, bg Color) GimpGradient {
	ggr.fg = fg
	ggr.bg = bg
	return ggr
}

// endpointColor resolves an endpoint color according to its color type.
func endpointColor(ctype endpointColorType, fixed, fg, bg Color) Color {
	switch ctype {
	case colorForeground:
		return fg
	case colorForegroundTransparent:
		return Rgb(fg.R, fg.G, fg.B, 0)
	case colorBackground:
		return bg
	case colorBackgroundTransparent:
		return Rgb(bg.R, bg.G, bg.B, 0)
	default:
		return fixed
	}
}

func (ggr GimpGradient) At(t float64) Color {
	if math.IsNaN(t) {
		return Color{A: 1}
	}

	lcolor := endpointColor(ggr.segments[0].lctype, ggr.segments[0].lcolor, ggr.fg, ggr.bg)

	if t <= ggr.min {
		return lcolor
	}

	last := ggr.segments[len(ggr.segments)-1]
	rcolor := endpointColor(last.rctype, last.rcolor, ggr.fg, ggr.bg)

	if t >= ggr.max {
		return rcolor
	}

	low := 0
	high := len(ggr.segments)
	mid := 0

	for low < high {
		mid = (low + high) / 2
		if t > ggr.segments[mid].rpos {
			low = mid + 1
		} else if t < ggr.segments[mid].lpos {
			high = mid
		} else {
			break
		}
	}

	seg := ggr.segments[mid]
	seg_len := seg.rpos - seg.lpos

	segLcolor := endpointColor(seg.lctype, seg.lcolor, ggr.fg, ggr.bg)
	segRcolor := endpointColor(seg.rctype, seg.rcolor, ggr.fg, ggr.bg)

	var middle float64
	var pos float64

	if seg_len < epsilon {
		middle = 0.5
		pos = 0.5
	} else {
		middle = (seg.mpos - seg.lpos) / seg_len
		pos = (t - seg.lpos) / seg_len
	}

	var f float64

	switch seg.blending {
	case linear:
		f = calc_linear_factor(middle, pos)
	case curved:
		if middle < epsilon {
			return segRcolor
		} else if math.Abs(1-middle) < epsilon {
			return segLcolor
		} else {
			f = math.Exp(-math.Ln2 * math.Log10(pos) / math.Log10(middle))
		}
	case sinusoidal:
		x := calc_linear_factor(middle, pos)
		f = (math.Sin(-fracPi2+math.Pi*x) + 1) / 2
	case sphericalIncreasing:
		x := calc_linear_factor(middle, pos) - 1
		f = math.Sqrt(1 - x*x)
	case sphericalDecreasing:
		x := calc_linear_factor(middle, pos)
		f = 1 - math.Sqrt(1-x*x)
	case step:
		if pos >= middle {
			return segRcolor
		} else {
			return segLcolor
		}
	}

	switch seg.coloring {
	case rgb:
		return blendRgb(segLcolor, segRcolor, f)
	case hsvCcw:
		return blendHsvCcw(segLcolor, segRcolor, f)
	case hsvCw:
		return blendHsvCw(segLcolor, segRcolor, f)
	}

	return lcolor
}

func calc_linear_factor(middle, pos float64) float64 {
	if pos <= middle {
		if middle < epsilon {
			return 0
		} else {
			return 0.5 * pos / middle
		}
	} else {
		pos = pos - middle
		middle = 1 - middle

		if middle < epsilon {
			return 1
		} else {
			return 0.5 + 0.5*pos/middle
		}
	}
}

func blendHsvCcw(c1, c2 Color, t float64) Color {
	hsvA := col2hsv(c1)
	hsvB := col2hsv(c2)

	var hue float64

	if hsvA[0] < hsvB[0] {
		hue = hsvA[0] + ((hsvB[0] - hsvA[0]) * t)
	} else {
		h := hsvA[0] + ((360 - (hsvA[0] - hsvB[0])) * t)

		if h > 360 {
			hue = h - 360
		} else {
			hue = h
		}
	}

	return Hsv(
		hue,
		hsvA[1]+t*(hsvB[1]-hsvA[1]),
		hsvA[2]+t*(hsvB[2]-hsvA[2]),
		hsvA[3]+t*(hsvB[3]-hsvA[3]),
	)
}

func blendHsvCw(c1, c2 Color, t float64) Color {
	hsvA := col2hsv(c1)
	hsvB := col2hsv(c2)

	var hue float64

	if hsvB[0] < hsvA[0] {
		hue = hsvA[0] - ((hsvA[0] - hsvB[0]) * t)
	} else {
		h := hsvA[0] - ((360 - (hsvB[0] - hsvA[0])) * t)

		if h < 0 {
			hue = h + 360
		} else {
			hue = h
		}
	}

	return Hsv(
		hue,
		hsvA[1]+t*(hsvB[1]-hsvA[1]),
		hsvA[2]+t*(hsvB[2]-hsvA[2]),
		hsvA[3]+t*(hsvB[3]-hsvA[3]),
	)
}

func ParseGgr(r io.Reader, fg, bg Color) (Gradient, string, error) {
	zgrad := Gradient{
		Core: zeroGradient{},
		Min:  0,
		Max:  1,
	}

	segments := []gimpSegment{}
	var nseg int
	var name string
	xseg := 0
	i := 0
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		if i == 0 {
			if scanner.Text() != "GIMP Gradient" {
				return zgrad, name, fmt.Errorf("invalid header")
			}
		} else if i == 1 {
			if !strings.HasPrefix(scanner.Text(), "Name:") {
				return zgrad, name, fmt.Errorf("invalid header")
			}

			name = strings.TrimSpace(scanner.Text()[5:])
		} else if i == 2 {
			t, ok := parseFloat(scanner.Text())

			if ok {
				nseg = int(t)
			} else {
				return zgrad, name, fmt.Errorf("invalid header")
			}
		} else {
			if i >= nseg+3 {
				break
			}

			seg, ok := parseSegment(scanner.Text())

			if ok {
				segments = append(segments, seg)
				xseg++
			} else {
				return zgrad, name, fmt.Errorf("invalid segment")
			}
		}
		i++
	}

	if err := scanner.Err(); err != nil {
		return zgrad, name, err
	}

	if len(segments) == 0 {
		return zgrad, name, fmt.Errorf("segments %v", i)
	}

	if xseg < nseg {
		return zgrad, name, fmt.Errorf("wrong segments count, %v, %v", nseg, xseg)
	}

	gradbase := GimpGradient{
		segments: segments,
		min:      0,
		max:      1,
		fg:       fg,
		bg:       bg,
		name:     name,
	}

	return Gradient{
		Core: gradbase,
		Min:  0,
		Max:  1,
	}, name, nil
}

func parseSegment(s string) (gimpSegment, bool) {
	params := strings.Fields(s)
	plen := len(params)

	if plen != 13 && plen != 15 {
		return gimpSegment{}, false
	}

	d := make([]float64, 15)

	for i, x := range params {
		t, ok := parseFloat(x)

		if ok {
			d[i] = t
			continue
		}

		return gimpSegment{}, false
	}

	if plen == 13 {
		d[13] = 0
		d[14] = 0
	}

	var blending blendingType

	switch int(d[11]) {
	case 0:
		blending = linear
	case 1:
		blending = curved
	case 2:
		blending = sinusoidal
	case 3:
		blending = sphericalIncreasing
	case 4:
		blending = sphericalDecreasing
	case 5:
		blending = step
	default:
		return gimpSegment{}, false
	}

	var coloring coloringType

	switch int(d[12]) {
	case 0:
		coloring = rgb
	case 1:
		coloring = hsvCcw
	case 2:
		coloring = hsvCw
	default:
		return gimpSegment{}, false
	}

	var lctype endpointColorType

	switch int(d[13]) {
	case 0:
		lctype = colorFixed
	case 1:
		lctype = colorForeground
	case 2:
		lctype = colorForegroundTransparent
	case 3:
		lctype = colorBackground
	case 4:
		lctype = colorBackgroundTransparent
	default:
		return gimpSegment{}, false
	}

	var rctype endpointColorType

	switch int(d[14]) {
	case 0:
		rctype = colorFixed
	case 1:
		rctype = colorForeground
	case 2:
		rctype = colorForegroundTransparent
	case 3:
		rctype = colorBackground
	case 4:
		rctype = colorBackgroundTransparent
	default:
		return gimpSegment{}, false
	}

	// Keep the raw color values from the file. They are the fixed endpoint
	// colors for fixed-type endpoints and are written back verbatim; dynamic
	// endpoints resolve through the gradient's foreground/background colors.
	return gimpSegment{
		lcolor:   Color{R: d[3], G: d[4], B: d[5], A: d[6]},
		rcolor:   Color{R: d[7], G: d[8], B: d[9], A: d[10]},
		lctype:   lctype,
		rctype:   rctype,
		lpos:     d[0],
		mpos:     d[1],
		rpos:     d[2],
		blending: blending,
		coloring: coloring,
	}, true
}

func isFiniteColor(c Color) bool {
	return math.IsNaN(c.R) == false && math.IsNaN(c.G) == false &&
		math.IsNaN(c.B) == false && math.IsNaN(c.A) == false &&
		math.IsInf(c.R, 0) == false && math.IsInf(c.G, 0) == false &&
		math.IsInf(c.B, 0) == false && math.IsInf(c.A, 0) == false
}

// validateGgrSegments checks the segments against the same rules GIMP enforces
// when loading a .ggr file: finite coordinates, left <= middle <= right,
// contiguous segments spanning the full [0, 1] range.
func validateGgrSegments(segments []gimpSegment) error {
	if len(segments) == 0 {
		return fmt.Errorf("ggr: gradient has no segments")
	}

	for i, seg := range segments {
		if math.IsNaN(seg.lpos) || math.IsNaN(seg.mpos) || math.IsNaN(seg.rpos) ||
			math.IsInf(seg.lpos, 0) || math.IsInf(seg.mpos, 0) || math.IsInf(seg.rpos, 0) {
			return fmt.Errorf("ggr: segment %d has non-finite coordinates", i)
		}

		if seg.lpos > seg.mpos || seg.mpos > seg.rpos {
			return fmt.Errorf("ggr: segment %d coordinates out of order: %v <= %v <= %v expected",
				i, seg.lpos, seg.mpos, seg.rpos)
		}

		if seg.blending < linear || seg.blending > step {
			return fmt.Errorf("ggr: segment %d has invalid blending type %d", i, seg.blending)
		}

		if seg.coloring < rgb || seg.coloring > hsvCw {
			return fmt.Errorf("ggr: segment %d has invalid coloring type %d", i, seg.coloring)
		}

		if seg.lctype < colorFixed || seg.lctype > colorBackgroundTransparent {
			return fmt.Errorf("ggr: segment %d has invalid left endpoint color type %d", i, seg.lctype)
		}

		if seg.rctype < colorFixed || seg.rctype > colorBackgroundTransparent {
			return fmt.Errorf("ggr: segment %d has invalid right endpoint color type %d", i, seg.rctype)
		}

		if seg.lctype == colorFixed && !isFiniteColor(seg.lcolor) {
			return fmt.Errorf("ggr: segment %d left endpoint color is not finite", i)
		}

		if seg.rctype == colorFixed && !isFiniteColor(seg.rcolor) {
			return fmt.Errorf("ggr: segment %d right endpoint color is not finite", i)
		}
	}

	if segments[0].lpos != 0 {
		return fmt.Errorf("ggr: first segment must start at 0, got %v", segments[0].lpos)
	}

	for i := 1; i < len(segments); i++ {
		if segments[i-1].rpos != segments[i].lpos {
			return fmt.Errorf("ggr: segments %d and %d are not contiguous: %v != %v",
				i-1, i, segments[i-1].rpos, segments[i].lpos)
		}
	}

	if segments[len(segments)-1].rpos != 1 {
		return fmt.Errorf("ggr: last segment must end at 1, got %v",
			segments[len(segments)-1].rpos)
	}

	return nil
}

// WriteGgr writes the gradient to w in the GIMP gradient (.ggr) file format.
//
// The gradient must have been produced by [ParseGgr] (i.e. its Core is a
// [GimpGradient]). The gradient name, segment intervals, midpoints, blending
// and coloring modes, alpha values and foreground/background endpoint types
// are preserved; fixed endpoint colors are written with their original values.
//
// An error is returned for gradients that are not GIMP gradients, invalid
// segment ordering or non-finite values, as well as for write failures.
func WriteGgr(w io.Writer, grad Gradient) error {
	ggr, ok := grad.Core.(GimpGradient)

	if !ok {
		return fmt.Errorf("ggr: gradient is not a GIMP gradient, cannot write .ggr")
	}

	if err := validateGgrSegments(ggr.segments); err != nil {
		return err
	}

	if strings.ContainsAny(ggr.name, "\r\n") {
		return fmt.Errorf("ggr: gradient name must not contain line breaks")
	}

	bw := bufio.NewWriter(w)
	write := func(format string, args ...any) error {
		if _, err := fmt.Fprintf(bw, format, args...); err != nil {
			return fmt.Errorf("ggr: write failed: %w", err)
		}
		return nil
	}

	if err := write("GIMP Gradient\nName: %s\n%d\n", ggr.name, len(ggr.segments)); err != nil {
		return err
	}

	for _, seg := range ggr.segments {
		// %g gives the shortest round-trippable decimal representation,
		// matching GIMP's g_ascii_dtostr output (always locale-independent
		// in Go) for values like 0, 0.5 and 1.
		line := "%g %g %g %g %g %g %g %g %g %g %g %d %d %d %d\n"
		args := []any{
			seg.lpos, seg.mpos, seg.rpos,
			seg.lcolor.R, seg.lcolor.G, seg.lcolor.B, seg.lcolor.A,
			seg.rcolor.R, seg.rcolor.G, seg.rcolor.B, seg.rcolor.A,
			int(seg.blending), int(seg.coloring),
			int(seg.lctype), int(seg.rctype),
		}

		if err := write(line, args...); err != nil {
			return err
		}
	}

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("ggr: write failed: %w", err)
	}

	return nil
}
