package config

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

type Config struct {
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
	ServerPort       string
	JWTSecret        string
}

func LoadConfig() Config {
	return Config{
		DatabaseHost:     getEnv("DATABASE_HOST", "db"),
		DatabasePort:     getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:     getEnv("DATABASE_USER", "postgres"),
		DatabasePassword: getEnv("DATABASE_PASSWORD", "password"),
		DatabaseName:     getEnv("DATABASE_NAME", "shop"),
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		JWTSecret:        getEnv("JWT_SECRET", "secret"),
	}
}

func LoadConfigOrPanic() Config {
	return LoadConfig()
}

func (c Config) PostgresConnStr() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DatabaseHost,
		c.DatabasePort,
		c.DatabaseUser,
		c.DatabasePassword,
		c.DatabaseName,
	)
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func InitDB(ctx context.Context, cfg Config) *sql.DB {
	db, err := sql.Open("postgres", cfg.PostgresConnStr())
	if err != nil {
		panic(fmt.Sprintf("Ошибка подключения к БД: %v", err))
	}
	if err = db.PingContext(ctx); err != nil {
		panic(fmt.Sprintf("Ошибка пинга БД: %v", err))
	}
	return db
}
