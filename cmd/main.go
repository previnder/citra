package main

import (
	"database/sql"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/previnder/citra"
)

func main() {
	// necessary for luid package
	rand.Seed(time.Now().UnixNano())

	configFile := flag.String("config", "", "Config file path")
	runMigrations := flag.Bool("migrate", false, "Run migrations")
	runServer := flag.Bool("serve", false, "Run HTTP server")
	dbUser := flag.String("db-user", "", "Database user")
	dbPassword := flag.String("db-pass", "", "Database password")
	dbName := flag.String("db", "", "Database name")
	addr := flag.String("addr", "", "Address to start the HTTP server on")
	uploadsDir := flag.String("uploads-dir", "", "Root uploads directory")
	flag.Parse()

	path := "./config.json"
	if *configFile != "" {
		path = *configFile
	}
	config, err := citra.UnmarshalConfigFile(path)
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
	if *addr != "" {
		config.Addr = *addr
	}
	if *uploadsDir != "" {
		config.RootUploadsDir = *uploadsDir
	}

	if config.DeletedDir != "" {
		info, err := os.Stat(config.DeletedDir)
		if err == nil {
			if !info.IsDir() {
				log.Fatal("Deleted images directory is not a directory")
			}
		} else {
			if os.IsNotExist(err) {
				if err = os.MkdirAll(config.DeletedDir, 0755); err != nil {
					log.Fatal("Error creating deleted images folder: ", err)
				}
			} else {
				log.Fatal("Error reading deleted dir: ", err)
			}
		}
	}

	db := openDB(config.Database.User, config.Database.Password, config.Database.Database)

	if *runMigrations {
		log.Println("Running migrations...")
		if err := citra.RunMigrations(db); err != nil {
			log.Fatal("Error running migrations: ", err)
		}
		log.Println("Migrations completed")
	}

	if *runServer {
		server := citra.NewServer(db, config)
		log.Println("Starting HTTP server on", config.Addr)
		log.Fatal(http.ListenAndServe(config.Addr, server))
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
