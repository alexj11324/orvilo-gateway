package gateway

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	JWKSPublicKey    string
	OrviloAPIBaseURL string
	Port             string
	ReadTimeout      time.Duration
	ServiceToken     string
	ShutdownTimeout  time.Duration
	WriteTimeout     time.Duration
}

func (c Config) Validate() error {
	if c.ServiceToken == "" {
		return errors.New("SERVICE_TOKEN is required")
	}
	return validateAPIKeyServerURL(c.OrviloAPIBaseURL, c.OrviloAPIBaseURL)
}

func ConfigFromEnv() Config {
	return Config{
		JWKSPublicKey:    os.Getenv("JWKS_PUBLIC_KEY"),
		OrviloAPIBaseURL: backendURLFromEnv(),
		Port:             envOrDefault("PORT", "8787"),
		ReadTimeout:      durationEnvOrDefault("READ_TIMEOUT", 30*time.Second),
		ServiceToken:     os.Getenv("SERVICE_TOKEN"),
		ShutdownTimeout:  durationEnvOrDefault("SHUTDOWN_TIMEOUT", 10*time.Second),
		WriteTimeout:     durationEnvOrDefault("WRITE_TIMEOUT", time.Minute),
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// backendURLFromEnv reads the Orvilo server address.
func backendURLFromEnv() string {
	return os.Getenv("ORVILO_API_BASE_URL")
}
