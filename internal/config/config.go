package config

import "os"

// Config holds runtime configuration loaded from environment variables.
// Keep this struct as the single source of truth for app settings so
// nothing reads os.Getenv directly outside this package.
type Config struct {
	Port   string
	DBPath string
	Env    string
}

func Load() Config {
	return Config{
		Port:   getEnv("PORT", "8080"),
		DBPath: getEnv("DB_PATH", "data.db"),
		Env:    getEnv("APP_ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
