package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type RedisConfig struct {
	URL string
	DB  int
}

type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// Load reads all configuration from environment variables.
// Returns an error listing every missing required variable so the operator can
// fix them all at once rather than discovering them one by one.
func Load() (*Config, error) {
	var missing []string

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			URL:             require("DATABASE_URL"),
			MaxConns:        int32(getInt("DB_MAX_CONNS", 10)),
			MinConns:        int32(getInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime: getDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
			DB:  getInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     require("JWT_SECRET"),
			Expiration: getDuration("JWT_EXPIRATION", 24*time.Hour),
		},
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
