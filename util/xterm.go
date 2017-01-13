package util

import (
	"fmt"
	"strconv"
)

var cubelevels = []uint64{0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff}
var midpoints = []uint64{0x2f, 0x73, 0x9b, 0xc3, 0xeb}

// RGB2xterm converts RGB value into xterm ANSI color sequence.
func RGB2xterm(rgb string) string {
	if len(rgb) != 6 {
		return ""
	}

	r, _ := strconv.ParseUint(rgb[0:2], 16, 8)
	g, _ := strconv.ParseUint(rgb[2:4], 16, 8)
	b, _ := strconv.ParseUint(rgb[4:6], 16, 8)

	if r+g+b > 500 {
		// decrease color brightness for dark terminal
		if r > 100 {
			r -= 20
		} else {
			r = 0
		}

		if g > 100 {
			g -= 20
		} else {
			g = 0
		}

		if b > 100 {
			b -= 20
		} else {
			b = 0
		}
	}

	rx := 0
	gx := 0
	bx := 0

	for _, v := range midpoints {
		if v < r {
			rx++
		}
		if v < g {
			gx++
		}
		if v < b {
			bx++
		}
	}

	return fmt.Sprintf("38;5;%d", rx*36+gx*6+bx+16)
}
