package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/bimg"
	"github.com/previnder/citra/pkg/luid"
)

const (
	// MaxImagesPerFolder is the maximum number of image files in one folder.
	// Copies are not counted.
	MaxImagesPerFolder = 5000
)

// Errors.
var (
	ErrUnsupportedImage = errors.New("unsupported image format")
)

// RGB represents color values of range (0, 255).
type RGB struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// ImageType represents a time of image.
type ImageType string

// List of image types.
const (
	ImageTypeJPEG = ImageType("jpeg")
	ImageTypeWEBP = ImageType("webp")
)

// ImageSize represents the size of an image.
type ImageSize struct {
	Width, Height int
}

// String implements Stringer interface.
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

// UnmarshalText implements TextUnmarshaler interface.
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

// ImageFit describes how an image is to be fitten into a rectangle.
type ImageFit string

// Valid ImageFit values.
const (
	ImageFitCover   = ImageFit("cover")
	ImageFitContain = ImageFit("contain")
)

// UnmarshalText implements TextUnmarshaler interface.
func (i *ImageFit) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case string(ImageFitCover), string(ImageFitContain):
		*i = ImageFit(str)
		return nil
	}
	return errors.New("invalid imagefit")
}

// ImageCopy is a copy of an image.
type ImageCopy struct {
	Width     int    `json:"w"`
	Height    int    `json:"h"`
	MaxWidth  int    `json:"mw"`
	MaxHeight int    `json:"mh"`
	ImageFit  string `json:"if"`
}

func (c ImageCopy) Filename(imageID string) string {
	return imageID + "_" + strconv.Itoa(c.MaxWidth) + "_" + strconv.Itoa(c.MaxHeight) + "_" + strings.ToLower(c.ImageFit) + ".jpg"
}

// DBImage is a record in images table.
type DBImage struct {
	ID       luid.ID `json:"id"`
	FolderID int     `json:"-"`

	// JPEG for now.
	Type ImageType `json:"type"`

	// Actual width of image.
	Width int `json:"width"`

	// Actual height of image.
	Height int `json:"height"`

	// This is the MaxWidth that was provided as an argument
	// to addImage API call.
	MaxWidth int `json:"maxWidth"`

	MaxHeight int `json:"maxHeight"`

	Size         int `json:"size"`
	UploadedSize int `json:"-"`

	AverageColor RGB `json:"averageColor"`

	// Copies are stored on disk (in appropriate folder) with
	// filename {ID}_{MaxWidth}_{MaxHeight}_{ImageFit}.jpg
	// Copies may be nil.
	Copies []*ImageCopy `json:"copies"`

	CreatedAt time.Time  `json:"createdAt"`
	IsDeleted bool       `json:"-"`
	DeletedAt *time.Time `json:"-"`

	// Is of the format /imgs/{FolderID}/{ID}.jpg.
	DefaultImageURL string `json:"defaultImageURL,omitempty"`
}

type saveImageArg struct {
	MaxWidth  int    `json:"maxWidth"`
	MaxHeight int    `json:"maxHeight"`
	ImageFit  string `json:"imageFit"`
	IsDefault bool   `json:"default"`
}

// SaveImage saves image in buf to disk and creates a record in images table.
// It also creates thumbnails of thumb sizes.
//
// The images are saved as JPEGs.
func SaveImage(db *sql.DB, buf []byte, copies []saveImageArg, rootDir string) (*DBImage, error) {
	var defaultCopy saveImageArg
	for _, item := range copies {
		if item.IsDefault {
			defaultCopy = item
		}
	}

	originalWidth, originalHeight, err := imageSize(buf)
	if err != nil {
		return nil, err
	}

	if !defaultCopy.IsDefault {
		return nil, errors.New("no default image as an argument")
	}

	jpg, size, err := compressImageJPEG(buf, defaultCopy.MaxWidth, defaultCopy.MaxHeight, defaultCopy.ImageFit == "cover")
	if err != nil {
		if strings.Contains(err.Error(), "Unsupported image format") {
			return nil, ErrUnsupportedImage
		}
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	folderID, err := createImagesFolder(tx, rootDir)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	ID, now := luid.New()
	folder := filepath.Join(rootDir, strconv.Itoa(folderID))

	// save and save copies.
	var savedCopies []*ImageCopy
	var containSizes []ImageSize // saved contain images
	if defaultCopy.ImageFit == "contain" {
		containSizes = append(containSizes, ImageSize{size.Width, size.Height})
	}
	if err = ioutil.WriteFile(filepath.Join(folder, ID.String()+".jpg"), jpg, 0755); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, item := range copies {
		if item.IsDefault {
			continue
		}
		if item.ImageFit == "contain" {
			w, h := fitToResolution(originalWidth, originalHeight, item.MaxWidth, item.MaxHeight)
			skip := false
			for _, size := range containSizes {
				if size.Width == w && size.Height == h {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		c, err := saveImageCopy(buf, item, folder, ID.String())
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		savedCopies = append(savedCopies, c)
		if item.ImageFit == "contain" {
			containSizes = append(containSizes, ImageSize{c.Width, c.Height})
		}
	}

	// calculate image prominent color.
	jpegImage, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	color, _ := json.Marshal(imageAverageColor(jpegImage))

	savedCopiesJSON, _ := json.Marshal(savedCopies)

	_, err = tx.Exec(`insert into images (id, folder_id, width, height,
		maxWidth, maxHeight, type, size, uploaded_size, copies, average_color, created_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ID, folderID, size.Width, size.Height, defaultCopy.MaxWidth, defaultCopy.MaxHeight,
		ImageTypeJPEG, len(jpg), len(buf), savedCopiesJSON, color, now)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	return nil, tx.Commit()
}

// folder is rootDir/folderID and it already exists.
func saveImageCopy(buf []byte, arg saveImageArg, folder, imageID string) (*ImageCopy, error) {
	jpeg, size, err := compressImageJPEG(buf, arg.MaxWidth, arg.MaxHeight, arg.ImageFit == "cover")
	if err != nil {
		if strings.Contains(err.Error(), "Unsupported image format") {
			return nil, ErrUnsupportedImage
		}
		return nil, err
	}

	c := &ImageCopy{
		MaxWidth:  arg.MaxWidth,
		MaxHeight: arg.MaxHeight,
		Width:     size.Width,
		Height:    size.Height,
		ImageFit:  arg.ImageFit,
	}

	if err = ioutil.WriteFile(filepath.Join(folder, c.Filename(imageID)), jpeg, 0755); err != nil {
		return nil, err
	}

	return c, nil
}

// createImagesFolder creates a folder on disk and a record on folders table if
// no folders are available or returns the last folder id.
func createImagesFolder(tx *sql.Tx, rootDir string) (int, error) {
	var folderID, imagesCount int
	createFolder := false

	row := tx.QueryRow("select id, images_count from folders order by id desc limit 1")
	if err := row.Scan(&folderID, &imagesCount); err != nil {
		if err == sql.ErrNoRows {
			createFolder = true
		} else {
			return 0, err
		}
	}

	if imagesCount >= MaxImagesPerFolder {
		createFolder = true
	}

	if !createFolder {
		return folderID, nil
	}

	res, err := tx.Exec("insert into folders () values ()")
	if err != nil {
		return 0, err
	}

	ID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(ID), os.MkdirAll(filepath.Join(rootDir, strconv.Itoa(int(ID))), 0755)
}

// imageSize returns the size of image in buf (of any image type supported by
// libvips).
func imageSize(buf []byte) (w int, h int, err error) {
	image := bimg.NewImage(buf)
	size, err := image.Size()
	if err != nil {
		return
	}
	w, h = size.Width, size.Height
	return
}
