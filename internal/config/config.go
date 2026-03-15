package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type AppConfig struct {
	Telegram TelegramConfig
	Logging  LoggingConfig
	Server   ServerConfig
	Kafka    KafkaConfig
	Database Database
	JWT      JWTConfig
	AIConfig AIConfig
}

type ServerConfig struct {
	Port    string
	BaseURL string
}

type TelegramConfig struct {
	APIID    int32
	APIHash  string
	LogLevel int32
	TestDc   bool
}

type LoggingConfig struct {
	Level  string
	Format string
}

type KafkaConfig struct {
	Broker           string `yaml:"broker" env:"KAFKA_BROKER" envSeparator:","`
	ChatUpdatesTopic string `yaml:"topic_updates" env:"KAFKA_CHAT_UPDATES_TOPIC" envDefault:"chat-updates"`
	UserActionsTopic string `yaml:"topic_updates" env:"KAFKA_USER_ACTIONS_TOPIC" envDefault:"user-actions"`
	GroupID          string `yaml:"group_id" env:"KAFKA_GROUP_ID" envDefault:"reaction-telegram"`
}

type Database struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type JWTConfig struct {
	SecretKey     string        `env:"JWT_SECRET_KEY,required"`
	TokenDuration time.Duration `env:"JWT_TOKEN_DURATION" envDefault:"24h"`
}

type AIConfig struct {
	APIKey         string `yaml:"api_key" env:"YANDEX_AI_API_KEY"`
	FolderID       string `yaml:"folder_id" env:"YANDEX_AI_FOLDER_ID"`
	YandexIamToken string `yaml:"folder_id" env:"YANDEX_IAM_TOKEN"`
}

func Load() (*AppConfig, error) {
	cfg := &AppConfig{}

	apiID, err := strconv.Atoi(GetEnv("TELEGRAM_API_ID", ""))
	if err != nil {
		return nil, err
	}
	cfg.Telegram.APIID = int32(apiID)
	cfg.Telegram.APIHash = GetEnv("TELEGRAM_API_HASH", "")

	logLevel, err := strconv.Atoi(GetEnv("LOG_LEVEL", ""))
	if err != nil {
		return nil, err
	}
	cfg.Telegram.LogLevel = int32(logLevel)

	cfg.Telegram.TestDc = GetEnvAsBool("TEST_DC", false)

	log.Println("TEST_DC=", cfg.Telegram.TestDc)

	cfg.Server.Port = GetEnv("SERVER_HTTP_PORT", "8080")
	cfg.Server.BaseURL = GetEnv("SERVER_BASE_URL", "https://api.re-action.site")

	cfg.Kafka.Broker = GetEnv("KAFKA_BROKER", "kafka:9092")
	if topic := os.Getenv("KAFKA_USER_ACTIONS_TOPIC"); topic != "" {
		cfg.Kafka.ChatUpdatesTopic = topic
	}
	if topic := os.Getenv("KAFKA_CHAT_UPDATES_TOPIC"); topic != "" {
		cfg.Kafka.UserActionsTopic = topic
	}
	cfg.Kafka.GroupID = "reaction-telegram"

	cfg.Database = Database{
		Host:     GetEnv("DB_HOST", ""),
		Port:     GetEnv("DB_PORT", ""),
		User:     GetEnv("DB_USER", ""),
		Password: GetEnv("DB_PASSWORD", ""),
		DBName:   GetEnv("DB_NAME", ""),
	}

	jwtDuration, err := time.ParseDuration(GetEnv("JWT_TOKEN_DURATION", "24h"))
	if err != nil {
		jwtDuration = 24 * time.Hour
	}

	cfg.JWT = JWTConfig{
		SecretKey:     GetEnv("JWT_SECRET_KEY", "very-very-secret-key"),
		TokenDuration: jwtDuration,
	}

	cfg.AIConfig = AIConfig{
		APIKey:         GetEnv("YANDEX_AI_API_KEY", ""),
		FolderID:       GetEnv("YANDEX_AI_FOLDER_ID", ""),
		YandexIamToken: GetEnv("YANDEX_IAM_TOKEN", ""),
	}

	return cfg, nil
}

func MustLoad() *AppConfig {
	cfg, err := Load()
	if err != nil {
		log.Println("Failed to load configuration: %v\n", err)
		log.Println("\nPlease set the following environment variables:")
		log.Println("  TELEGRAM_API_ID     - Your Telegram API ID")
		log.Println("  TELEGRAM_API_HASH   - Your Telegram API Hash")
		log.Println("\nYou can create a .env file with these variables")
		panic(err)
	}
	return cfg
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	value = strings.ToLower(value)
	return value == "true"
}
