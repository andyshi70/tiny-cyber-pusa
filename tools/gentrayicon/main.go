// gentrayicon writes internal/tray/assets/trayTemplate.png: a macOS menu-bar
// "template image" — black pixels whose alpha is the coverage of the blue
// gate glyph from the app icon, on a transparent background. macOS renders a
// template image as a mask, so the same bytes read correctly in a light or a
// dark menu bar and invert while the menu is open.
//
//	go run ./tools/gentrayicon assets/icon-src.png internal/tray/assets/trayTemplate.png
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

// out is the menu-bar icon in points×2 (macOS menu bar is 22pt on notch-less
// and 24pt on notched displays; 44px covers both at Retina).
const out = 44

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gentrayicon <src.png> <out.png>")
		os.Exit(2)
	}
	src, err := decode(os.Args[1])
	if err != nil {
		panic(err)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, out, out))
	draw.Draw(dst, dst.Bounds(), image.Transparent, image.Point{}, draw.Src)

	b := src.Bounds()
	for y := 0; y < out; y++ {
		for x := 0; x < out; x++ {
			if cov := coverage(src, b, x, y, out); cov > 0 {
				dst.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(cov*255 + 0.5)})
			}
		}
	}

	f, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, dst); err != nil {
		panic(err)
	}
}

// coverage box-filters the destination pixel over the source rectangle it maps
// to, averaging how much of the blue glyph each sample covers.
func coverage(src *image.NRGBA, b image.Rectangle, x, y, size int) float64 {
	sx0 := b.Min.X + x*b.Dx()/size
	sx1 := b.Min.X + (x+1)*b.Dx()/size
	sy0 := b.Min.Y + y*b.Dy()/size
	sy1 := b.Min.Y + (y+1)*b.Dy()/size
	var sum float64
	var n int
	for sy := sy0; sy < sy1; sy++ {
		for sx := sx0; sx < sx1; sx++ {
			if sx < b.Min.X || sy < b.Min.Y || sx >= b.Max.X || sy >= b.Max.Y {
				continue
			}
			sum += ink(src.NRGBAAt(sx, sy))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// ink is how much of a source pixel belongs to the gate glyph. The app icon is
// a flat #1a1a1a rounded plate carrying a blue arch and a pale bar, so
// "brighter than the plate" separates the glyph cleanly; the partial-coverage
// ramp keeps the arch's gradient from turning into a stepped edge. The plate
// alpha is folded in because its rounded corners are antialiased.
func ink(c color.NRGBA) float64 {
	if c.A == 0 {
		return 0
	}
	level := max3(c.R, c.G, c.B)
	var v float64
	switch {
	case level >= 96:
		v = 1
	case level <= 40:
		v = 0
	default:
		v = float64(level-40) / 56
	}
	return v * float64(c.A) / 255
}

func max3(v ...uint8) int {
	m := int(v[0])
	for _, x := range v[1:] {
		if int(x) > m {
			m = int(x)
		}
	}
	return m
}

func decode(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	in, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(in.Bounds())
	draw.Draw(out, out.Bounds(), in, in.Bounds().Min, draw.Src)
	return out, nil
}
