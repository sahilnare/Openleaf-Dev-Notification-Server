package main

import (
	"Openleaf-Dev-B2B-Appointment-Server/config"
	"Openleaf-Dev-B2B-Appointment-Server/cron"
	"Openleaf-Dev-B2B-Appointment-Server/log"
	"Openleaf-Dev-B2B-Appointment-Server/server"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [cron|webhook|test|test-phase2|test-reminder|test-all]")
		os.Exit(1)
	}

	mode := os.Args[1]
	config.LoadEnv()
	log.InitLogger()

	switch mode {
	case "cron":
		cron.RunCronJob()
	case "webhook":
		server.StartWebhookServer()
	case "test":
		// TODO: Add test appointment and run cron
		fmt.Println("Test mode not yet implemented")
	case "test-phase2":
		fmt.Println("Test phase2 mode not yet implemented")
	case "test-reminder":
		fmt.Println("Test reminder mode not yet implemented")
	case "test-all":
		fmt.Println("Test all mode not yet implemented")
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
	}
} 