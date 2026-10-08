package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the validated environment variables
type Config struct {
	LivekitURL       string
	LivekitAPIKey    string
	LivekitAPISecret string
	TTSAPIKey        string
	DeepgramAPIKey   string
	GroqAPIKey       string

	// Guitar co-pilot settings (all optional with sensible defaults)

	// HTTPPort is the port for the embedded client web server. Default: "8080".
	HTTPPort string
	// RMSGateDBFS is the minimum RMS level (in dBFS) for Gate 1 of chord
	// analysis. Signals below this are treated as dead strums. Default: "-40".
	RMSGateDBFS string
	// TuningToleranceCents is the maximum acceptable tuning offset in cents
	// before Zeplin warns the student to retune. Default: "20".
	TuningToleranceCents string
}

// Load verifies and loads required configuration variables from a .env file and environment
func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Could not load .env file from root directory: %v", err)
	}

	requiredKeys := []string{
		"LIVEKIT_URL",
		"LIVEKIT_API_KEY",
		"LIVEKIT_API_SECRET",
		"TTS_API_KEY",
		"DEEPGRAM_API_KEY",
		"GROQ_API_KEY",
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

	httpPort := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if httpPort == "" {
		httpPort = "8080"
	}
	rmsGate := strings.TrimSpace(os.Getenv("RMS_GATE_DBFS"))
	if rmsGate == "" {
		rmsGate = "-40"
	}
	tuningTol := strings.TrimSpace(os.Getenv("TUNING_TOLERANCE_CENTS"))
	if tuningTol == "" {
		tuningTol = "20"
	}

	return &Config{
		LivekitURL:           strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
		LivekitAPIKey:        strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
		LivekitAPISecret:     strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
		TTSAPIKey:            strings.TrimSpace(os.Getenv("TTS_API_KEY")),
		DeepgramAPIKey:       strings.TrimSpace(os.Getenv("DEEPGRAM_API_KEY")),
		GroqAPIKey:           strings.TrimSpace(os.Getenv("GROQ_API_KEY")),
		HTTPPort:             httpPort,
		RMSGateDBFS:          rmsGate,
		TuningToleranceCents: tuningTol,
	}
}
