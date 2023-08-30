package citra

import (
	"errors"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/h2non/bimg"
)

// ImageType represents the type of image.
type ImageType string

// List of image types.
const (
	ImageTypeJPEG = ImageType("jpeg")
	ImageTypeWEBP = ImageType("webp")
)

// Errors.
var (
	ErrInvalidImageFit  = errors.New("invalid image fit")
	ErrUnsupportedImage = errors.New("unsupported image format")
	ErrNoImage          = errors.New("image buffer empty")
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

// String returns, for example, "400" if width and height are both 400px, and
// "400x600" if width is 400px and height is 600px.
func (s ImageSize) String() string {
	if s.Width == s.Height {
		return strconv.Itoa(s.Width)
	}
	return strconv.Itoa(s.Width) + "x" + strconv.Itoa(s.Height)
}

// MarshalText implements encoding.TextMarshaler interface. Output same as
// String method.
func (s ImageSize) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface. It does the
// reverse of what MarshalText and String do.
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
	// ImageFitCover covers the given container with the image. The resulting
	// image may be shrunken, enlarged, and/or cropped.
	ImageFitCover = ImageFit("cover")

	// ImageFitContain fits the image in the container without either enlarging
	// the image or cropping it.
	ImageFitContain = ImageFit("contain")

	ImageFitDefault = ImageFitContain
)

// UnmarshalText implements encoding.TextUnmarshaler interface.
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
	return ErrInvalidImageFit
}

// ContainInResolution returns width and height as they fit into an image of
// size w and h. Aspect ratio is not changed.
func ContainInResolution(width, height, w, h int) (int, int) {
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

// AverageColor returns the average RGB color of img by averaging the colors of
// at most 10000 pixels. Each RGB value is in the range of (0,255).
func AverageColor(img image.Image) RGB {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	xsteps, ysteps := int(math.Floor(float64(width)/100.0)), int(math.Floor(float64(height)/100.0))
	if xsteps <= 0 {
		xsteps = 1
	}
	if ysteps <= 0 {
		ysteps = 1
	}

	var r, g, b float64
	for i := 0; i < width; i += xsteps {
		for j := 0; j < height; j += ysteps {
			c := img.At(i, j)
			r2, g2, b2, _ := c.RGBA()
			r = (r + float64(r2)) / 2
			g = (g + float64(g2)) / 2
			b = (b + float64(b2)) / 2
		}
	}

	var c RGB
	x := r + g + b
	if x > 0 {
		c.R = int(r / x * 255.0)
		c.G = int(g / x * 255.0)
		c.B = int(b / x * 255.0)
	}
	return c
}

// ToJPEG converts the image to a JPEG, if it's not already, and fits the image
// into maxWidth and maxHeight according to fit.
func ToJPEG(image []byte, maxWidth, maxHeight int, fit ImageFit) ([]byte, ImageSize, error) {
	s := ImageSize{}
	img := bimg.NewImage(image)
	if img.Type() != bimg.ImageTypeName(bimg.JPEG) {
		if _, err := img.Convert(bimg.JPEG); err != nil {
			return nil, s, bimgError(err)
		}
	}

	size, err := img.Size()
	if err != nil {
		return nil, s, bimgError(err)
	}

	var w, h int
	if fit == ImageFitCover {
		w, h = maxWidth, maxHeight
	} else if fit == ImageFitContain {
		w, h = ContainInResolution(size.Width, size.Height, maxWidth, maxHeight)
	} else {
		return nil, s, ErrInvalidImageFit
	}

	s.Width, s.Height = w, h
	image, err = img.ResizeAndCrop(w, h)
	return image, s, bimgError(err)
}

func bimgError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "unsupported image format") {
		return ErrUnsupportedImage
	}
	return err
}

// GetImageSize returns the size of image.
func GetImageSize(image []byte) (w int, h int, err error) {
	img := bimg.NewImage(image)
	size, err := img.Size()
	if err != nil {
		err = bimgError(err)
		return
	}
	w, h = size.Width, size.Height
	return
}
