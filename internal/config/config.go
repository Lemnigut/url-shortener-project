package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	envprovider "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config — корневая структура конфигурации приложения.
type Config struct {
	App      AppConfig      `koanf:"app"`
	Postgres PostgresConfig `koanf:"postgres"`
}

// AppConfig — настройки приложения.
type AppConfig struct {
	Port          string `koanf:"port"`
	BaseURL       string `koanf:"base_url"`
	SnowflakeNode int64  `koanf:"snowflake_node"`
}

// PostgresConfig — параметры подключения к PostgreSQL.
type PostgresConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	DBName   string `koanf:"db_name"`
	SSLMode  string `koanf:"ssl_mode"`
}

// DSN формирует строку подключения к PostgreSQL.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode,
	)
}

// Load загружает конфигурацию из YAML-файла (путь задаётся через CONFIG_PATH),
// затем переопределяет значения из переменных окружения.
// По умолчанию: internal/config/configs/local.yaml
func Load() *Config {
	k := koanf.New(".")

	// 1. Загрузка YAML-файла конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "internal/config/configs/local.yaml"
	}

	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		log.Fatalf("конфиг: не удалось загрузить %s: %v", configPath, err)
	}

	// 2. Переопределение переменными окружения
	//    POSTGRES_PASSWORD -> postgres.password
	//    APP_BASE_URL      -> app.base_url
	//    APP_PORT          -> app.port
	k.Load(envprovider.Provider(".", envprovider.Opt{
		Prefix: "",
		TransformFunc: func(key, value string) (string, any) {
			key = strings.ToLower(key)

			mapping := map[string]string{
				"app_port":           "app.port",
				"app_base_url":       "app.base_url",
				"app_snowflake_node": "app.snowflake_node",
				"postgres_host":      "postgres.host",
				"postgres_port":      "postgres.port",
				"postgres_user":      "postgres.user",
				"postgres_password":  "postgres.password",
				"postgres_db_name":   "postgres.db_name",
				"postgres_ssl_mode":  "postgres.ssl_mode",
			}

			if mapped, ok := mapping[key]; ok {
				return mapped, value
			}
			return "", nil // неизвестные переменные игнорируются
		},
	}), nil)

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		log.Fatalf("конфиг: ошибка десериализации: %v", err)
	}

	return &cfg
}
