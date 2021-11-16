package main

import (
	"image"
	"math"

	"github.com/h2non/bimg"
)

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
func imageAverageColor(img image.Image) RGB {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	xsteps, ysteps := int(math.Ceil(float64(width)/100.0)), int(math.Ceil(float64(height)/100.0))
	var r, g, b uint32

	for i := 0; i < width; i += xsteps {
		for j := 0; j < height; j += ysteps {
			c := img.At(i, j)
			r2, g2, b2, _ := c.RGBA()
			r = (r + r2) / 2
			g = (g + g2) / 2
			b = (b + b2) / 2
		}
	}

	var c RGB
	x := float64(r + g + b)
	c.R = int(float64(r) / x * 255)
	c.G = int(float64(g) / x * 255)
	c.B = int(float64(b) / x * 255)
	return c
}

// compressImageJPEG converts the image in buf to a JPEG and, if cover is
// false, fits the image into maxWidth and maxHeight without changing aspect
// ratio, otherwise the images size is set to maxWidth and maxHeight and the
// image is to cover the whole canvas. It returns the processed images width
// and height.
func compressImageJPEG(buf []byte, maxWidth, maxHeight int, cover bool) ([]byte, ImageSize, error) {
	s := ImageSize{}
	image := bimg.NewImage(buf)
	if image.Type() != bimg.ImageTypeName(bimg.JPEG) {
		if _, err := image.Convert(bimg.JPEG); err != nil {
			return nil, s, err
		}
	}

	size, err := image.Size()
	if err != nil {
		return nil, s, err
	}

	var w, h int
	if cover {
		w, h = maxWidth, maxHeight
	} else {
		w, h = fitToResolution(size.Width, size.Height, maxWidth, maxHeight)
	}

	s.Width, s.Height = w, h
	buf, err = image.ResizeAndCrop(w, h)
	return buf, s, err
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
