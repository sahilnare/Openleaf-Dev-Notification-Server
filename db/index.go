package db

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var GlobalDB *sqlx.DB = nil

func Connect() *sqlx.DB {

	host, name, user, password, port := GetDBConfig()

	db, err := sqlx.Connect("postgres", fmt.Sprintf("user=%s dbname=%s sslmode=disable password=%s host=%s port=%s", user, name, password, host, port))
	if err != nil {
		log.Println("unable to connect to database", err.Error())
		os.Exit(1)
	}

	err = db.Ping()
	if err != nil {
		log.Println("database connection failed", err.Error())
		os.Exit(1)
	}

	GlobalDB = db
	return db
}

func GetDB() *sqlx.DB {
	return GlobalDB
}
