package main

import (
	"log"

	"github.com/Abhinav-Rao24/Zeplin/config"
)

func main() {
	log.Println("Starting Zeplin Voice AI Framework...")
	
	// Initialize configuration
	cfg := config.Load()
	
	log.Println("Configuration loaded successfully.")
	_ = cfg // Keep compiler happy
}
