package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// config is the application wide configuration struct.
type config struct {
	// MariaDB connection.
	Database struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`

	// HTTP port to listen on.
	Port int `json:"port"`

	// All images are saved inside subfolders in this directory.
	RootUploadsDir string `json:"rootUploadsDir"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	configFile := flag.String("config", "", "Config file path")
	runMigrations := flag.Bool("migrate", false, "Run migrations")
	runServer := flag.Bool("serve", false, "Run HTTP server")
	dbUser := flag.String("db-user", "", "Database user")
	dbPassword := flag.String("db-pass", "", "Database password")
	dbName := flag.String("db", "", "Database name")
	port := flag.Int("port", -1, "HTTP port to listen on")
	uploadsDir := flag.String("uploads-dir", "", "Root uploads directory")
	flag.Parse()

	path := "./config.json"
	if *configFile != "" {
		path = *configFile
	}
	config, err := unmarshalConfigFile(path)
	if err != nil {
		log.Fatal("Error reading config file: ", err)
	}

	// cmd args take precedence
	if *dbUser != "" {
		config.Database.User = *dbUser
	}
	if *dbPassword != "" {
		config.Database.Password = *dbPassword
	}
	if *dbName != "" {
		config.Database.Database = *dbName
	}
	if *port != -1 {
		config.Port = *port
	}
	if *uploadsDir != "" {
		config.RootUploadsDir = *uploadsDir
	}

	db := openDB(config.Database.User, config.Database.Password, config.Database.Database)

	if *runMigrations {
		log.Println("Running migrations...")
		if err := runDBMigrations(db); err != nil {
			log.Fatal("Error running migrations: ", err)
		}
		log.Println("Migrations completed")
	}

	if *runServer {
		server := newServer(db, config)
		log.Fatal(http.ListenAndServe("localhost:"+strconv.Itoa(config.Port), server))
	}
}

// openDB opens a database connection to mysql or mariadb.
func openDB(user, password, database string) *sql.DB {
	db, err := sql.Open("mysql", user+":"+password+"@/"+database+"?parseTime=true")
	if err != nil {
		log.Fatal("Error connecting to mysql: ", err)
	}
	return db
}

// returns a default config in case file is missing.
func unmarshalConfigFile(file string) (*config, error) {
	config := &config{}
	config.Port = 3881
	config.RootUploadsDir = "./uploads"

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

// runDBMigrations runs all the migrations in migrations folder.
func runDBMigrations(db *sql.DB) error {
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
