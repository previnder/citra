package main

import (
	"errors"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/h2non/bimg"
)

// Errors.
var (
	ErrUnsupportedImage = errors.New("unsupported image format")
)

// ImageType represents a type of image.
type ImageType string

// List of image types.
const (
	ImageTypeJPEG = ImageType("jpeg")
	ImageTypeWEBP = ImageType("webp")
)

// RGB represents color values of range (0, 255).
type RGB struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// ImageSize represents the size of an image.
type ImageSize struct {
	Width, Height int
}

// String implements Stringer interface.
//
// It returns, as an example, "400" if width and height are both 400px, and
// "400x600" is width is 400px and height is 600px.
func (s ImageSize) String() string {
	if s.Width == s.Height {
		return strconv.Itoa(s.Width)
	}
	return strconv.Itoa(s.Width) + "x" + strconv.Itoa(s.Height)
}

// MarshalText implements TextMarshaler interface.
func (s ImageSize) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements TextUnmarshaler interface. It does the reverse of
// what MarshalText and String do.
func (s *ImageSize) UnmarshalText(text []byte) error {
	str := string(text)
	i := strings.Index(str, "x")
	if i == -1 {
		width, err := strconv.Atoi(str)
		if err != nil {
			return err
		}
		s.Width = width
		s.Height = width
		return nil
	}

	if len(str) < i+2 {
		return errors.New("invalid image size")
	}

	width, err := strconv.Atoi(str[:i])
	if err != nil {
		return err
	}
	height, err := strconv.Atoi(str[i+1:])
	if err != nil {
		return err
	}

	s.Width, s.Height = width, height
	return nil
}

// ImageFit denotes how an image is to be fitted into a rectangle.
type ImageFit string

// Valid ImageFit values.
const (
	ImageFitCover   = ImageFit("cover")
	ImageFitContain = ImageFit("contain")
	ImageFitDefault = ImageFitContain
)

// UnmarshalText implements TextUnmarshaler interface.
func (i *ImageFit) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case string(ImageFitCover), string(ImageFitContain):
		*i = ImageFit(str)
		return nil
	case "":
		*i = ImageFitDefault
		return nil
	}
	return errors.New("invalid ImageFit")
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

// compressImageJPEG converts the image in buf to a JPEG, if it's not already,
// and fits the image into maxWidth and maxHeight as per fit.
func compressImageJPEG(buf []byte, maxWidth, maxHeight int, fit ImageFit) ([]byte, ImageSize, error) {
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
	if fit == ImageFitCover {
		w, h = maxWidth, maxHeight
	} else if fit == ImageFitContain {
		w, h = fitToResolution(size.Width, size.Height, maxWidth, maxHeight)
	} else {
		return nil, s, errors.New("invalid ImageFit")
	}

	s.Width, s.Height = w, h
	buf, err = image.ResizeAndCrop(w, h)
	return buf, s, err
}

// imageSize returns the size of image in buf.
func imageSize(buf []byte) (w int, h int, err error) {
	image := bimg.NewImage(buf)
	size, err := image.Size()
	if err != nil {
		return
	}
	w, h = size.Width, size.Height
	return
}
