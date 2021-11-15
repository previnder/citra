package main

import (
	"image"
	"math"

	"github.com/h2non/bimg"
)

// RGB represents color values of range (0, 255).
type RGB struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// fitToResolution returns width and height as they fit into an image of size w
// and h. Aspect ratio is not changed.
func fitToResolution(width, height, w, h int) (int, int) {
	x, y, scale := float64(width), float64(height), 1.0
	if width > w {
		scale = float64(w) / float64(width)
		x = scale * float64(width)
		y = scale * float64(height)
	}
	if y > float64(h) {
		scale = float64(h) / y
		x = scale * x
		y = scale * y
	}
	return int(x), int(y)
}

// imageAverageColor returns the average RGB color values of img by averaging
// the colors of at most 10000 pixels. Each RGB value is in the range of
// (0,255).
func imageAverageColor(img image.Image) (r, g, b uint32) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	xsteps, ysteps := int(math.Ceil(float64(width)/100.0)), int(math.Ceil(float64(height)/100.0))

	for i := 0; i < width; i += xsteps {
		for j := 0; j < height; j += ysteps {
			c := img.At(i, j)
			r2, g2, b2, _ := c.RGBA()
			r = (r + r2) / 2
			g = (g + g2) / 2
			b = (b + b2) / 2
		}
	}

	x := float64(r + g + b)
	r = uint32(float64(r) / x * 255)
	g = uint32(float64(g) / x * 255)
	b = uint32(float64(b) / x * 255)
	return
}

// compressImageJPEG converts the image in buf to a JPEG and fits the image
// into maxWidth and maxHeight without changing aspect ratio.
func compressImageJPEG(buf []byte, maxWidth, maxHeight int) ([]byte, error) {
	image := bimg.NewImage(buf)
	if image.Type() != bimg.ImageTypeName(bimg.JPEG) {
		if _, err := image.Convert(bimg.JPEG); err != nil {
			return nil, err
		}
	}

	size, err := image.Size()
	if err != nil {
		return nil, err
	}

	w, h := fitToResolution(size.Width, size.Height, maxWidth, maxHeight)
	return image.ResizeAndCrop(w, h)
}

// compressImageWEBP converts the image in buf to a WEBP and fits the image
// into maxWidth and maxHeight without changing aspect ratio.
func compressImageWEBP(buf []byte, maxWidth, maxHeight int) ([]byte, error) {
	image := bimg.NewImage(buf)
	if image.Type() != bimg.ImageTypeName(bimg.WEBP) {
		if _, err := image.Convert(bimg.WEBP); err != nil {
			return nil, err
		}
	}

	size, err := image.Size()
	if err != nil {
		return nil, err
	}

	w, h := fitToResolution(size.Width, size.Height, maxWidth, maxHeight)
	return image.ResizeAndCrop(w, h)
}
