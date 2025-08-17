package db

import (
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var GlobalDB *sqlx.DB = nil

func Connect() *sqlx.DB {

	db, err := sqlx.Connect("postgres", "user=openleaf dbname=b2b-productiondb sslmode=disable password=geralt1212leaf host=localhost port=63333")
	// db, err := sqlx.Connect("postgres", "user=openleaf dbname=b2b-productiondb sslmode=disable password=geralt1212leaf host=database-openleaf-1.cyuh3uofyiy4.ap-south-1.rds.amazonaws.com port=5432")
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
