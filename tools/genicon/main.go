// genicon writes internal/tray/assets/tray.ico: a 32×32 PNG wrapped in the
// ICO container format (Vista+ accepts PNG-compressed icon entries).
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
)

const size = 32

func main() {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{75, 111, 255, 255}
	fg := color.RGBA{255, 255, 255, 255}
	transparent := color.RGBA{0, 0, 0, 0}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := transparent
			if inRounded(x, y) {
				c = bg
			}
			img.Set(x, y, c)
		}
	}
	// abstract white mark: a thick stroke + short tail
	for y := 12; y < 18; y++ {
		for x := 7; x < 25; x++ {
			img.Set(x, y, fg)
		}
	}
	for y := 20; y < 25; y++ {
		for x := 18; x < 25; x++ {
			img.Set(x, y, fg)
		}
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		panic(err)
	}
	var ico bytes.Buffer
	ico.Write([]byte{0, 0, 1, 0, 1, 0}) // ICONDIR: type=icon, count=1
	ico.Write([]byte{size, size, 0, 0}) // ICONDIRENTRY: w, h, colors, reserved
	ico.Write([]byte{1, 0, 32, 0})      // planes, bitcount
	writeUint32(&ico, uint32(pngBuf.Len()))
	writeUint32(&ico, 22)
	ico.Write(pngBuf.Bytes())
	if err := os.WriteFile(os.Args[1], ico.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

func inRounded(x, y int) bool {
	radius := 7
	if x < 0 || y < 0 || x >= size || y >= size {
		return false
	}
	nearLeft, nearRight := x < radius, x >= size-radius
	nearTop, nearBottom := y < radius, y >= size-radius
	if (nearLeft || nearRight) && (nearTop || nearBottom) {
		cx := radius
		if nearRight {
			cx = x - (size - 1 - radius)
		} else {
			cx = radius - x
		}
		cy := radius
		if nearBottom {
			cy = y - (size - 1 - radius)
		} else {
			cy = radius - y
		}
		return cx*cx+cy*cy <= radius*radius
	}
	return true
}

func writeUint32(b *bytes.Buffer, v uint32) {
	b.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}
