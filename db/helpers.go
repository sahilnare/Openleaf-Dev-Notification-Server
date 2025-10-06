package db

import "os"

func GetDBConfig() (string, string, string, string, string) {
	host := os.Getenv("DB_HOST")
	name := os.Getenv("DB_NAME")
	user := os.Getenv("DB_USERNAME")
	password := os.Getenv("DB_PASSWORD")
	port := os.Getenv("DB_PORT")

	if host == "" || name == "" || user == "" || password == "" || port == "" {
		panic("One or more required DB environment variables are not set")
	}

	return host, name, user, password, port
}



