package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type config struct {
	Database struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`
}

func main() {
	configFile := flag.String("config", "", "Config file path")
	runMigrations := flag.Bool("migrate", false, "Run migrations")
	flag.Parse()

	path := "./config.json"
	if *configFile != "" {
		path = *configFile
	}
	config, err := unmarshalConfigFile(path)
	if err != nil {
		log.Fatal("Could not read config file: ", err)
	}

	db := openDB(config.Database.User, config.Database.Password, config.Database.Database)

	if *runMigrations {
		log.Println("Running migrations...")
		if err := runDBMigrations(db); err != nil {
			log.Fatal("Error running migrations: ", err)
		}
		log.Println("Migrations completed")
	}
}

func openDB(user, password, database string) *sql.DB {
	db, err := sql.Open("mysql", user+":"+password+"@/"+database+"?parseTime=true")
	if err != nil {
		log.Fatal("Error connecting to mysql: ", err)
	}
	return db
}

func unmarshalConfigFile(file string) (*config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	config := &config{}
	if err = json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

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
