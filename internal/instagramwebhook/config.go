package instagramwebhook

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	MetaWebhookVerifyToken string
}

func LoadConfig(path string) (Config, error) {
	values, err := readDotEnv(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	cfg := Config{
		Port:                   firstNonEmpty(os.Getenv("PORT"), values["PORT"]),
		DatabaseURL:            firstNonEmpty(os.Getenv("DATABASE_URL"), values["DATABASE_URL"]),
		MetaWebhookVerifyToken: firstNonEmpty(os.Getenv("META_WEBHOOK_VERIFY_TOKEN"), values["META_WEBHOOK_VERIFY_TOKEN"]),
	}

	if cfg.Port == "" {
		cfg.Port = "9090"
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if cfg.MetaWebhookVerifyToken == "" {
		return Config{}, errors.New("META_WEBHOOK_VERIFY_TOKEN is required")
	}

	return cfg, nil
}

func readDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
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
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key != "" {
			values[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}
