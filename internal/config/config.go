package config

import (
	"log"
	"net"
	"net/url"
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
		cfg.DatabaseURL = defaultDatabaseURL()
		log.Println("DATABASE_DSN not set, using default PostgreSQL URL (override with DATABASE_DSN)")
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

// defaultDatabaseURL builds a DSN for the bundled docker-compose Postgres service.
// DATABASE_DSN, when set, takes precedence over these parts.
func defaultDatabaseURL() string {
	host := getEnvOrDefault("DATABASE_HOST", "postgres")
	user := getEnvOrDefault("POSTGRES_USER", "tasktracker")
	password := getEnvOrDefault("POSTGRES_PASSWORD", "tasktracker")
	dbname := getEnvOrDefault("POSTGRES_DB", "tasktracker")
	port := getEnvOrDefault("POSTGRES_PORT", "5432")
	sslmode := getEnvOrDefault("PGSSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + dbname,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}
