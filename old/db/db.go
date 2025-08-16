package db

import (
	"Openleaf-Dev-B2B-Appointment-Server/config"
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	cfg := config.ConfigData
	connStr := "host=" + cfg.DBHost + " port=" + cfg.DBPort + " user=" + cfg.DBUser + " password=" + cfg.DBPassword + " dbname=" + cfg.DBName + " sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("DB ping failed: %v", err)
	}
	DB = db
	log.Println("[OK] Connected to PostgreSQL")
}

func GetDB() *sql.DB {
	return DB
} 