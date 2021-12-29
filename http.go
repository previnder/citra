package citra

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/previnder/citra/pkg/luid"
)

// Server is an HTTP server that processes and serves images.
type Server struct {
	db     *sql.DB
	router *mux.Router
	config *Config
}

// NewServer returns a new image server.
func NewServer(db *sql.DB, c *Config) *Server {
	s := &Server{}
	s.db = db
	s.config = c

	s.router = mux.NewRouter()

	s.router.Handle("/api/images", http.HandlerFunc(s.addImage)).Methods("POST")
	s.router.Handle("/api/images/_bulk", http.HandlerFunc(s.bulkDelete)).Methods("DELETE")
	s.router.Handle("/api/images/{imageID}", http.HandlerFunc(s.getImage)).Methods("GET")
	s.router.Handle("/api/images/{imageID}", http.HandlerFunc(s.deleteImage)).Methods("DELETE")

	s.router.NotFoundHandler = http.HandlerFunc(s.notFoundHandler)
	s.router.MethodNotAllowedHandler = http.HandlerFunc(s.methodNotAllowedHandler)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")
		s.router.ServeHTTP(w, r)
	} else {
		s.serveImages(w, r)
	}
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	res := struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		Status:  statusCode,
		Message: message,
	}

	w.WriteHeader(statusCode)
	data, _ := json.Marshal(res)
	w.Write(data)
}

func (s *Server) writeInternalServerError(w http.ResponseWriter, err error) {
	s.writeError(w, http.StatusInternalServerError, "Internal Server error")
	log.Println("500 error:", err)
	debug.PrintStack()
}

func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusNotFound, "Not found")
}

func (s *Server) methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

// abort if error is non-nil.
//
// If an error is encounted notFoundHandler is invoked.
func (s *Server) unmarshalLUID(w http.ResponseWriter, r *http.Request, ID string) (luid.ID, error) {
	var LUID luid.ID
	if err := LUID.UnmarshalText([]byte(ID)); err != nil {
		s.notFoundHandler(w, r)
		return LUID, err
	}

	return LUID, nil
}

// addImage accepts only POST requests and must have a content type of
// multipart/form-data.
func (s *Server) addImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // limit max upload size
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Error parsing multipart/form-data: "+err.Error())
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		s.writeInternalServerError(w, err)
		return
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		s.writeInternalServerError(w, err)
		return
	}

	copies := r.Form.Get("copies")
	if copies == "" {
		s.writeError(w, http.StatusBadRequest, "no copies to make")
		return
	}

	var args []SaveImageArg
	if err = json.Unmarshal([]byte(copies), &args); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	t1 := time.Now()

	image, err := SaveImage(s.db, buf, args, s.config.RootUploadsDir)
	if err != nil {
		if err == ErrNoDefaultImage {
			s.writeError(w, http.StatusBadRequest, "No default copy to make")
			return
		}
		if err == ErrUnsupportedImage {
			s.writeError(w, http.StatusBadRequest, "Unsupported image format")
			return
		}
		s.writeInternalServerError(w, err)
		return
	}

	log.Printf("Took %v to process %v\n", time.Since(t1), image.ID)

	data, _ := json.Marshal(image)
	w.Write(data)
}

func (s *Server) getImage(w http.ResponseWriter, r *http.Request) {
	imageID, err := s.unmarshalLUID(w, r, mux.Vars(r)["imageID"])
	if err != nil {
		return
	}

	image, err := GetImage(s.db, imageID)
	if err != nil {
		if err == sql.ErrNoRows {
			s.notFoundHandler(w, r)
			return
		}
		s.writeInternalServerError(w, err)
		return
	}

	data, _ := json.Marshal(image)
	w.Write(data)
}

func (s *Server) deleteImage(w http.ResponseWriter, r *http.Request) {
	imageID, err := s.unmarshalLUID(w, r, mux.Vars(r)["imageID"])
	if err != nil {
		return
	}

	image, err := DeleteImage(s.db, imageID, s.config.RootUploadsDir, s.config.DeletedDir)
	if err != nil {
		if err == sql.ErrNoRows {
			s.notFoundHandler(w, r)
			return
		}
		s.writeInternalServerError(w, err)
		return
	}

	data, _ := json.Marshal(image)
	w.Write(data)
}

// URL is of the form /images/{folderID/{imageID}.jpg[?size=1440x720&fit=cover]
func (s *Server) serveImages(w http.ResponseWriter, r *http.Request) {
	path := strings.Split(r.URL.Path, "/")
	if path[0] == "" {
		path = path[1:]
	}
	if len(path) != 3 {
		http.NotFound(w, r)
		return
	}
	if path[0] != "images" {
		http.NotFound(w, r)
		return
	}
	folderID, err := strconv.Atoi(path[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !strings.HasSuffix(path[2], ".jpg") {
		http.NotFound(w, r)
		return
	}
	imageID := luid.ID{}
	if err = imageID.UnmarshalText([]byte(path[2][:strings.LastIndex(path[2], ".jpg")])); err != nil {
		http.NotFound(w, r)
		return
	}

	q := r.URL.Query()
	name := imageID.String()
	if q.Get("size") != "" {
		var size ImageSize
		if err = size.UnmarshalText([]byte(q.Get("size"))); err != nil {
			http.NotFound(w, r)
			return
		}
		name += "_" + strconv.Itoa(size.Width) + "_" + strconv.Itoa(size.Height)
		fit := ImageFitContain
		if q.Get("fit") != "" {
			if err = fit.UnmarshalText([]byte(q.Get("fit"))); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		name += "_" + string(fit)
	}

	filepath := filepath.Join(s.config.RootUploadsDir, strconv.Itoa(folderID), name+".jpg")

	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		s.imageInternalServerError(w, r, err)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		s.imageInternalServerError(w, r, err)
		return
	}

	w.Header().Add("Cache-Control", "max-age=1209600, no-transform")
	w.Header().Add("Cross-Origin-Resource-Policy", "cross-origin")
	http.ServeContent(w, r, "", stat.ModTime(), file)
}

func (s *Server) imageInternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Println("500 error:", err)
	debug.PrintStack()
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Server error"))
}

func (s *Server) bulkDelete(w http.ResponseWriter, r *http.Request) {
	log.Println("Here mate!")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	var IDs []luid.ID
	if err = json.Unmarshal(data, &IDs); err != nil {
		s.writeError(w, http.StatusBadRequest, "Error reading JSON body")
		return
	}

	for _, id := range IDs {
		if _, err = DeleteImage(s.db, id, s.config.RootUploadsDir, s.config.DeletedDir); err != nil {
			s.imageInternalServerError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
