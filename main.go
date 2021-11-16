package main

import (
	"database/sql"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	config, err := UnmarshalConfigFile(path)
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

	db := openDB(config.Database.User, config.Database.Password, config.Database.Database)

	if *runMigrations {
		log.Println("Running migrations...")
		if err := RunMigrations(db); err != nil {
			log.Fatal("Error running migrations: ", err)
		}
		log.Println("Migrations completed")
	}

	if *runServer {
		server := NewServer(db, config)
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
