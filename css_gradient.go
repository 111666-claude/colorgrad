package colorgrad

import (
	"math"
	"strings"

	"github.com/mazznoer/csscolorparser"
)

func parseCss(s string) ([]cssGradientStop, bool) {
	stops, _, ok := parseCssEx(s)
	return stops, ok
}

// parseCssEx behaves like parseCss but additionally returns the indices of the
// standalone transition hint stops (position tokens without a color) in the
// returned slice. The hint stops still receive the same midpoint color as
// parseCss does, so existing callers see identical output.
func parseCssEx(s string) ([]cssGradientStop, []int, bool) {
	stops := []cssGradientStop{}
	hintIdx := []int{}

	for _, stop := range splitByComma(s) {
		n0 := len(stops)
		if !prosesStop(&stops, splitBySpace(stop)) {
			return stops, hintIdx, false
		}
		// A freshly appended stop without a color is a transition hint.
		for i := n0; i < len(stops); i++ {
			if stops[i].color == nil {
				hintIdx = append(hintIdx, i)
			}
		}
	}

	if len(stops) == 0 {
		return stops, hintIdx, false
	}

	if stops[0].color == nil {
		return stops, hintIdx, false
	}

	if stops[0].pos == nil {
		stops[0].pos = ptr(0.0)
	}

	for i, stop := range stops {
		if i == len(stops)-1 {
			if stop.pos == nil {
				stops[i].pos = ptr(1.0)
			}
			break
		}

		if stop.color == nil {
			if stops[i+1].color == nil {
				return stops, hintIdx, false
			}
			stops[i].color = ptrColor(blendRgb(*stops[i-1].color, *stops[i+1].color, 0.5))
		}
	}

	if *stops[0].pos > 0.0 {
		stops = append([]cssGradientStop{{ptr(0.0), stops[0].color}}, stops...)
		// Prepending one stop shifts every recorded hint index by one.
		for i := range hintIdx {
			hintIdx[i]++
		}
	}

	if *stops[len(stops)-1].pos < 1.0 {
		stops = append(stops, cssGradientStop{ptr(1.0), stops[len(stops)-1].color})
	}

	for i, stop := range stops {
		if stop.pos == nil {
			for j := i + 1; j < len(stops); j++ {
				if stops[j].pos != nil {
					prev := *stops[i-1].pos
					next := *stops[j].pos
					stops[i].pos = ptr(prev + (next-prev)/float64(j-i+1))
					break
				}
			}
		}

		if i > 0 {
			stops[i].pos = ptr(math.Max(*stops[i].pos, *stops[i-1].pos))
		}
	}

	for _, stop := range stops {
		if stop.color == nil || stop.pos == nil {
			return stops, hintIdx, false
		}
	}

	return stops, hintIdx, true
}

func ptr(f float64) *float64 {
	return &f
}

func ptrColor(c Color) *Color {
	return &c
}

type cssGradientStop struct {
	pos   *float64
	color *Color
}

func prosesStop(stops *[]cssGradientStop, arr []string) bool {
	switch len(arr) {
	case 1:
		col, err := csscolorparser.Parse(arr[0])
		if err == nil {
			*stops = append(*stops, cssGradientStop{nil, &col})
			return true
		}

		pos, ok := parsePos(arr[0])
		if ok {
			*stops = append(*stops, cssGradientStop{&pos, nil})
			return true
		}
		return false
	case 2:
		col, err := csscolorparser.Parse(arr[0])
		if err != nil {
			return false
		}

		pos, ok := parsePos(arr[1])
		if !ok {
			return false
		}

		*stops = append(*stops, cssGradientStop{&pos, &col})
	case 3:
		col, err := csscolorparser.Parse(arr[0])
		if err != nil {
			return false
		}

		pos1, ok1 := parsePos(arr[1])
		if !ok1 {
			return false
		}

		pos2, ok2 := parsePos(arr[2])
		if !ok2 {
			return false
		}

		*stops = append(*stops, cssGradientStop{&pos1, &col})
		*stops = append(*stops, cssGradientStop{&pos2, &col})
	default:
		return false
	}
	return true
}

func splitByComma(s string) []string {
	res := []string{}
	beg := 0
	inside := false

	for i := 0; i < len(s); i++ {
		if s[i] == ',' && !inside {
			res = append(res, s[beg:i])
			beg = i + 1
		} else if s[i] == '(' {
			inside = true
		} else if s[i] == ')' {
			inside = false
		}
	}
	return append(res, s[beg:])
}

func splitBySpace(s string) []string {
	res := []string{}
	beg := 0
	inside := false

	for i := 0; i < len(s); i++ {
		if s[i] == ' ' && !inside {
			if len(s[beg:i]) > 0 {
				res = append(res, s[beg:i])
			}
			beg = i + 1
		} else if s[i] == '(' {
			inside = true
		} else if s[i] == ')' {
			inside = false
		}
	}
	if len(s[beg:]) > 0 {
		res = append(res, s[beg:])
	}
	return res
}

func parsePos(s string) (float64, bool) {
	if strings.HasSuffix(s, "%") {
		f, ok := parseFloat(s[:len(s)-1])
		if ok && cssFinite(f) {
			return f / 100, true
		}
		return 0, false
	}

	f, ok := parseFloat(s)
	if !ok || !cssFinite(f) {
		return 0, false
	}
	return f, ok
}

func cssFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}
