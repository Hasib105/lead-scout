package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	TelegramBotToken   string
	TelegramChatID     string
	RedditClientID     string
	RedditClientSecret string
	RedditUserAgent    string
	NVIDIAAPIKey       string
	NVIDIABaseURL      string
	NVIDIAModel        string
	APIAddr            string
	HTTPTimeout        time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:        getenv("DATABASE_URL", "postgres://lead_scout:lead_scout@localhost:5432/lead_scout?sslmode=disable"),
		TelegramBotToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:     os.Getenv("TELEGRAM_CHAT_ID"),
		RedditClientID:     os.Getenv("REDDIT_CLIENT_ID"),
		RedditClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
		RedditUserAgent:    getenv("REDDIT_USER_AGENT", "lead-scout/0.1"),
		NVIDIAAPIKey:       os.Getenv("NVIDIA_API_KEY"),
		NVIDIABaseURL:      getenv("NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1/chat/completions"),
		NVIDIAModel:        getenv("NVIDIA_MODEL", "google/gemma-4-31b-it"),
		APIAddr:            getenv("API_ADDR", ":8080"),
		HTTPTimeout:        envDuration("HTTP_TIMEOUT", 20*time.Second),
	}
}

func LoadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func (c Config) ValidateDB() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	return nil
}

func (c Config) TelegramConfigured() bool {
	return c.TelegramBotToken != "" && c.TelegramChatID != ""
}

func (c Config) RedditConfigured() bool {
	return c.RedditClientID != "" && c.RedditClientSecret != "" && c.RedditUserAgent != ""
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if seconds, err := strconv.Atoi(v); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
