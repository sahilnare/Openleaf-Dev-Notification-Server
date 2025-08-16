package log

import (
	"log"
	"os"
)

var Logger *log.Logger

func InitLogger() {
	Logger = log.New(os.Stdout, "[Openleaf] ", log.LstdFlags|log.Lshortfile)
} 