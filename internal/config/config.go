package config

import "os"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL    string
	Port           string
	AdminToken     string
	ExamHMACSecret string
	CORSOrigins    string
}

// Load reads configuration from environment variables, using defaults for dev.
func Load() Config {
	return Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://quizforge:quizforge@localhost:5433/quizforge?sslmode=disable"),
		Port:           getEnv("PORT", "8080"),
		AdminToken:     getEnv("ADMIN_TOKEN", "dev-admin-token"),
		ExamHMACSecret: getEnv("EXAM_HMAC_SECRET", "dev-secret"),
		CORSOrigins:    getEnv("CORS_ORIGINS", "http://localhost:5173"),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
