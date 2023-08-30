package citra

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"image/jpeg"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	"github.com/previnder/citra/pkg/luid"
)

const (
	// MaxImagesPerFolder sets the maximum number of image files (without
	// counting copies) that can be saved in one folder.
	MaxImagesPerFolder = 4000
)

// Errors.
var (
	ErrNoDefaultImage = errors.New("no default image was provided")
)

// RunMigrations runs all the migrations in migrations folder.
func RunMigrations(db *sql.DB) error {
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "mysql", driver)
	if err != nil {
		return err
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		log.Println("Migrations no change")
		return nil
	}
	return err
}

// ImageCopy is a copy of an image.
type ImageCopy struct {
	Width     int      `json:"w"`
	Height    int      `json:"h"`
	MaxWidth  int      `json:"mw"`
	MaxHeight int      `json:"mh"`
	ImageFit  ImageFit `json:"if"`
	// Size of image in bytes.
	Size int `json:"s"`
}

// Filename returns the basename of the image stored on disk.
func (c ImageCopy) Filename(imageID string) string {
	return imageID + "_" + strconv.Itoa(c.MaxWidth) + "_" + strconv.Itoa(c.MaxHeight) + "_" + strings.ToLower(string(c.ImageFit)) + ".jpg"
}

// DBImage is a record in the images table.
type DBImage struct {
	ID luid.ID `json:"id"`

	FolderID int `json:"folderId"`

	// Always JPEG for now.
	Type ImageType `json:"type"`

	// Actual width of image.
	Width int `json:"width"`

	// Actual height of image.
	Height int `json:"height"`

	// This is the MaxWidth that was provided as an argument
	// to addImage API call.
	MaxWidth int `json:"maxWidth"`

	MaxHeight int `json:"maxHeight"`

	// Size of image in bytes.
	Size int `json:"size"`

	// Size of original uploaded image in bytes.
	UploadedSize int `json:"-"`

	AverageColor RGB `json:"averageColor"`

	// Copies are stored on disk (in appropriate folders) with filename
	// {ID}_{MaxWidth}_{MaxHeight}_{ImageFit}.jpg Copies may be nil.
	Copies []*ImageCopy `json:"copies"`

	CreatedAt time.Time  `json:"createdAt"`
	IsDeleted bool       `json:"deleted"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// URL pathname of the image. Is of the format /images/{FolderID}/{ID}.jpg.
	URL string `json:"url,omitempty"`

	// URL pathnames of the image and all its copies.
	URLs []string `json:"urls,omitempty"`
}

// GenerateURLs populates i.URL and i.URLs fields.
func (i *DBImage) GenerateURLs() {
	folderID := strconv.Itoa(i.FolderID)
	ID := i.ID.String()
	i.URL = "/images/" + folderID + "/" + ID + ".jpg"

	i.URLs = append(i.URLs, i.URL)
	for _, item := range i.Copies {
		q := "size=" + strconv.Itoa(item.MaxWidth) + "x" + strconv.Itoa(item.MaxHeight) + "&fit=" + string(item.ImageFit)
		i.URLs = append(i.URLs, i.URL+"?"+q)
	}
}

// SaveImageArg is an argument to SaveImage. It describes how a copy of the
// saving image is to be created.
type SaveImageArg struct {
	MaxWidth  int      `json:"maxWidth"`
	MaxHeight int      `json:"maxHeight"`
	ImageFit  ImageFit `json:"imageFit"`

	// If true, this is no longer a copy but the default, or the original,
	// image. There can be only one default copy per image (if multiple
	// arguments are provided as being default the first one is selected and
	// others are discarded).
	IsDefault bool `json:"default"`
}

// SaveImage saves the image in buf to disk (in a folder inside rootDir) and
// creates a record in images table. It also creates and stores copies of the
// image.
//
// All images are saved as JPEGs (for now).
func SaveImage(db *sql.DB, buf []byte, copies []SaveImageArg, rootDir string) (*DBImage, error) {
	if len(buf) == 0 {
		return nil, ErrNoImage
	}

	var defaultCopy SaveImageArg
	for _, item := range copies {
		if item.IsDefault {
			defaultCopy = item
		}
	}

	if !defaultCopy.IsDefault {
		return nil, ErrNoDefaultImage
	}

	jpg, size, err := ToJPEG(buf, defaultCopy.MaxWidth, defaultCopy.MaxHeight, defaultCopy.ImageFit)
	if err != nil {
		return nil, err
	}

	originalWidth, originalHeight, err := GetImageSize(buf)
	if err != nil {
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
	if defaultCopy.ImageFit == ImageFitContain {
		containSizes = append(containSizes, ImageSize{size.Width, size.Height})
	}
	if err = ioutil.WriteFile(filepath.Join(folder, ID.String()+".jpg"), jpg, 0755); err != nil {
		tx.Rollback()
		return nil, err
	}
	// Save copies to disk. ImageFit contain copies are skipped if a copy is
	// already saved with the same width and height.
	for _, item := range copies {
		if item.IsDefault {
			continue
		}
		if item.ImageFit == ImageFitContain {
			w, h := ContainInResolution(originalWidth, originalHeight, item.MaxWidth, item.MaxHeight)
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
		if item.ImageFit == ImageFitContain {
			containSizes = append(containSizes, ImageSize{c.Width, c.Height})
		}
	}

	// calculate image prominent color.
	jpegImage, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	color, _ := json.Marshal(AverageColor(jpegImage))

	savedCopiesJSON, _ := json.Marshal(savedCopies)

	_, err = tx.Exec(`insert into images (id, folder_id, width, height,
		max_width, max_height, type, size, uploaded_size, copies, average_color, created_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ID, folderID, size.Width, size.Height, defaultCopy.MaxWidth, defaultCopy.MaxHeight,
		ImageTypeJPEG, len(jpg), len(buf), savedCopiesJSON, color, now)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if _, err = tx.Exec("update folders set images_count = images_count + 1, total_size = total_size + ? where id = ?", len(jpg), folderID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return GetImage(db, ID)
}

// folder is rootDir/folderID and it already exists.
func saveImageCopy(buf []byte, arg SaveImageArg, folder, imageID string) (*ImageCopy, error) {
	jpeg, size, err := ToJPEG(buf, arg.MaxWidth, arg.MaxHeight, arg.ImageFit)
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
		Size:      len(jpeg),
	}

	if err = ioutil.WriteFile(filepath.Join(folder, c.Filename(imageID)), jpeg, 0755); err != nil {
		return nil, err
	}

	return c, nil
}

// createImagesFolder creates a folder on disk and a record on folders table if
// no folders are available (or if the folder is full) or returns the last
// folder id.
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

// GetImage returns an image from DB. It may return a deleted image.
func GetImage(db *sql.DB, ID luid.ID) (*DBImage, error) {
	st, err := db.Prepare(`select id, folder_id, type, width, height, max_width, max_height,
		size, uploaded_size, average_color, copies, created_at, is_deleted,
		deleted_at from images where id = ?`)
	if err != nil {
		return nil, err
	}

	row := st.QueryRow(ID)
	image := &DBImage{}
	var copies, color []byte

	err = row.Scan(&image.ID, &image.FolderID, &image.Type, &image.Width, &image.Height,
		&image.MaxWidth, &image.MaxHeight, &image.Size, &image.UploadedSize, &color,
		&copies, &image.CreatedAt, &image.IsDeleted, &image.DeletedAt)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(color, &image.AverageColor); err != nil {
		return nil, errors.New("error unmarshaling color: " + err.Error())
	}
	if err = json.Unmarshal(copies, &image.Copies); err != nil {
		return nil, errors.New("error unmarshaling copies: " + err.Error())
	}

	image.GenerateURLs()

	return image, nil
}

// DeleteImage sets is_deleted field of images to true. If deletedDir is
// non-empty, images are moved to that directory. Otherwise they are deleted.
func DeleteImage(db *sql.DB, ID luid.ID, rootDir, deletedDir string) (*DBImage, error) {
	image, err := GetImage(db, ID)
	if err != nil {
		return nil, err
	}

	if image.IsDeleted {
		return image, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	st, err := tx.Prepare("update images set is_deleted = ?, deleted_at = ? where id = ?")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	now := time.Now()
	if _, err = st.Exec(true, now, ID); err != nil {
		tx.Rollback()
		return nil, err
	}

	// copy original to deleted images folder
	prefix := image.ID.String()
	if deletedDir != "" {
		originalPath := filepath.Join(rootDir, strconv.Itoa(image.FolderID), prefix+".jpg")
		data, err := ioutil.ReadFile(originalPath)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if err = ioutil.WriteFile(filepath.Join(deletedDir, prefix+".jpg"), data, 0755); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// delete files on disk
	if _, err = deleteFilesByPrefix(filepath.Join(rootDir, strconv.Itoa(image.FolderID)), prefix); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	image.DeletedAt = &now
	image.IsDeleted = true

	return image, nil
}

// deleteFilesByPrefix deletes all files in dir with filename prefix s and
// returns the number of files deleted. If an error is encounted no of files
// deleted up to that point is returned.
func deleteFilesByPrefix(dir, s string) (int, error) {
	file, err := os.Open(dir)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	names, err := file.Readdirnames(0)
	if err != nil {
		return 0, err
	}

	n := 0
	for _, name := range names {
		if strings.HasPrefix(name, s) {
			if err = os.Remove(filepath.Join(dir, name)); err != nil {
				return n, err
			}
			n++
		}
	}

	return n, nil
}
