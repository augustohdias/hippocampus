package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	OllamaModel          string
	OllamaHost           string
	QdrantHost           string
	QdrantCollection     string
	LogLevel             string
	SearchScoreThreshold float32
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		OllamaModel:          getEnv("OLLAMA_MODEL", "qwen3-embedding:4b"),
		OllamaHost:           getEnv("OLLAMA_HOST", ""),
		QdrantHost:           getEnv("QDRANT_HOST", "localhost:6334"),
		QdrantCollection:     getEnv("QDRANT_COLLECTION", "memories"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		SearchScoreThreshold: getEnvFloat32("SEARCH_SCORE_THRESHOLD", 0.65),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFloat32(key string, defaultValue float32) float32 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 32); err == nil {
			return float32(f)
		}
	}
	return defaultValue
}
