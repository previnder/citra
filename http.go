package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type server struct {
	db     *sql.DB
	router *mux.Router
	config *config
}

func newServer(db *sql.DB) *server {
	s := &server{}
	s.db = db

	s.router = mux.NewRouter()

	s.router.Handle("/api/images", http.HandlerFunc(s.addImage)).Methods("POST")
	s.router.Handle("/api/image/{imageID}", http.HandlerFunc(s.getImage)).Methods("GET")
	s.router.Handle("/api/image/{imageID}", http.HandlerFunc(s.deleteImage)).Methods("DELETE")

	s.router.NotFoundHandler = http.HandlerFunc(s.notFoundHandler)
	s.router.MethodNotAllowedHandler = http.HandlerFunc(s.methodNotAllowedHandler)

	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")
		s.router.ServeHTTP(w, r)
	} else {
		// file server
	}
}

func (s *server) writeError(w http.ResponseWriter, statusCode int, message string) {
	res := struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		Status:  statusCode,
		Message: message,
	}

	data, _ := json.Marshal(res)
	w.WriteHeader(statusCode)
	w.Write(data)
}

func (s *server) writeInternalServerError(w http.ResponseWriter, err error) {
	s.writeError(w, http.StatusInternalServerError, "Internal server error")
	log.Println(err)
}

func (s *server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusNotFound, "Not found")
}

func (s *server) methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

// addImage accepts only POST requests and must have a content type of
// multipart/form-data.
func (s *server) addImage(w http.ResponseWriter, r *http.Request) {
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

	args := []saveImageArg{
		{MaxHeight: 1440, MaxWidth: 720, ImageFit: "contain"},
		{MaxHeight: 2160, MaxWidth: 1080, ImageFit: "contain"},
		{MaxHeight: 4320, MaxWidth: 2160, ImageFit: "contain"},
		{MaxHeight: 5000, MaxWidth: 5000, ImageFit: "contain", IsDefault: true},
		// {MaxHeight: 720, MaxWidth: 1280, ImageFit: "cover"},
	}

	t1 := time.Now()
	_, err = SaveImage(s.db, buf, args, "./uploads")
	log.Println(time.Since(t1))
	if err != nil {
		log.Println(err)
	}
}

func (s *server) getImage(w http.ResponseWriter, r *http.Request) {
}

func (s *server) deleteImage(w http.ResponseWriter, r *http.Request) {
}

func (s *server) serveImages(w http.ResponseWriter, r *http.Request) {
}
