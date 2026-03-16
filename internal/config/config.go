package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	JWTSecret     string
	AdminEmail    string
	AdminPassword string
	Port          string
	APKDir        string // directory containing task-tracker-latest.apk for GET /apk/latest
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	cfg := &Config{
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminEmail:    getEnvOrDefault("ADMIN_EMAIL", "john@veilstream.com"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		Port:          getEnvOrDefault("PORT", "3000"),
		APKDir:        getEnvOrDefault("APK_DIR", "apk"),
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_DSN")
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_DSN is required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if cfg.AdminPassword == "" {
		log.Fatal("ADMIN_PASSWORD is required")
	}

	return cfg
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
