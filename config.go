package citra

import (
	"encoding/json"
	"io"
	"os"
)

// Config is the configuration for the HTTP server.
type Config struct {
	// MariaDB connection.
	Database struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`

	// Address to listen on.
	Addr string `json:"addr"`

	// All images are saved inside subfolders in this directory.
	RootUploadsDir string `json:"rootUploadsDir"`

	// Deleted images are moved here. If DeletedDir is empty, the images are
	// deleted.
	DeletedDir string `json:"deletedDir"`
}

// UnmarshalConfigFile reads the config in file and returns it. In case no such
// file is found, it returns a default config.
func UnmarshalConfigFile(file string) (*Config, error) {
	config := &Config{}
	config.Addr = "localhost:3881"
	config.RootUploadsDir = "./uploads"
	config.DeletedDir = "./deleted"

	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}
