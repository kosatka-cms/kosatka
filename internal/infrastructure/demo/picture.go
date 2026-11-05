package demo

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
)

// palettes - the demo pictures' colours, from the deep to the shallows.
var palettes = [][2]color.RGBA{
	{{7, 20, 40, 255}, {34, 211, 238, 255}},    // abyss to cyan
	{{12, 74, 110, 255}, {94, 234, 212, 255}},  // ocean to aqua
	{{15, 23, 42, 255}, {20, 184, 166, 255}},   // night to teal
	{{30, 58, 138, 255}, {125, 211, 252, 255}}, // deep blue to sky
	{{8, 47, 73, 255}, {56, 189, 248, 255}},    // midnight to lagoon
	{{19, 78, 74, 255}, {153, 246, 228, 255}},  // kelp to foam
	{{49, 46, 129, 255}, {103, 232, 249, 255}}, // twilight to glow
	{{2, 6, 23, 255}, {148, 163, 184, 255}},    // orca black to mist
}

// Picture - the i-th picture as a JPEG: a gradient of its palette from
// the deep (bottom left) to the shallows, a soft glow and a few waves.
func Picture(w, h, i int) ([]byte, error) {
	p := palettes[i%len(palettes)]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)*(0.25+0.15*float64(i%4)), float64(h)*0.35
	phase := float64(i) * 0.9
	for y := range h {
		for x := range w {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			t := (fx + (1 - fy)) / 2
			glow := math.Max(0, 0.32-math.Hypot(float64(x)-cx, float64(y)-cy)/float64(w)) * 1.3
			wave := 0.0
			for k := 1; k <= 3; k++ {
				line := 0.55 + 0.12*float64(k) + 0.03*math.Sin(fx*math.Pi*2*float64(k+1)+phase)
				if d := math.Abs(fy - line); d < 0.004 {
					wave += 0.12 * (1 - d/0.004)
				}
			}
			mix := func(a, b uint8) uint8 {
				return uint8(math.Min(255, float64(a)*(1-t)+float64(b)*t+255*(glow+wave)))
			}
			img.SetRGBA(x, y, color.RGBA{mix(p[0].R, p[1].R), mix(p[0].G, p[1].G), mix(p[0].B, p[1].B), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
