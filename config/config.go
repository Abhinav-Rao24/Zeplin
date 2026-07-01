package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the validated environment variables
type Config struct {
	GeminiAPIKey     string
	LivekitURL       string
	LivekitAPIKey    string
	LivekitAPISecret string
	TTSAPIKey        string
}

// Load verifies and loads required configuration variables from a .env file and environment
func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Could not load .env file from root directory: %v", err)
	}

	requiredKeys := []string{
		"GEMINI_API_KEY",
		"LIVEKIT_URL",
		"LIVEKIT_API_KEY",
		"LIVEKIT_API_SECRET",
		"TTS_API_KEY",
	}

	var missingKeys []string
	
	for _, key := range requiredKeys {
		val := os.Getenv(key)
		if strings.TrimSpace(val) == "" {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		log.Fatalf("CRITICAL ERROR: The following required environment variables are missing or blank:\n- %s\n\nPlease ensure they are set in the .env file or environment.", strings.Join(missingKeys, "\n- "))
	}

	return &Config{
		GeminiAPIKey:     strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		LivekitURL:       strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
		LivekitAPIKey:    strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
		LivekitAPISecret: strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
		TTSAPIKey:        strings.TrimSpace(os.Getenv("TTS_API_KEY")),
	}
}
