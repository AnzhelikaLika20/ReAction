package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type AppConfig struct {
	Telegram TelegramConfig
	Logging  LoggingConfig
	Server   ServerConfig
	Kafka    KafkaConfig
	Database Database
	JWT      JWTConfig
}

type ServerConfig struct {
	Port string
}

type TelegramConfig struct {
	APIID   int32
	APIHash string
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

func Load() (*AppConfig, error) {
	cfg := &AppConfig{}

	apiID, err := strconv.Atoi(GetEnv("TELEGRAM_API_ID", ""))
	if err != nil {
		return nil, err
	}
	cfg.Telegram.APIID = int32(apiID)
	cfg.Telegram.APIHash = GetEnv("TELEGRAM_API_HASH", "")

	cfg.Logging.Level = GetEnv("LOG_LEVEL", "info")
	cfg.Logging.Format = GetEnv("LOG_FORMAT", "text")

	cfg.Server.Port = GetEnv("SERVER_HTTP_PORT", "8080")

	cfg.Kafka.Broker = GetEnv("KAFKA_BROKER", "kafka:9092")
	if topic := os.Getenv("KAFKA_USER_ACTIONS_TOPIC"); topic != "" {
		cfg.Kafka.ChatUpdatesTopic = topic
	}
	if topic := os.Getenv("KAFKA_CHAT_UPDATES_TOPIC"); topic != "" {
		cfg.Kafka.UserActionsTopic = topic
	}
	cfg.Kafka.GroupID = "reaction-telegram"

	cfg.Database = Database{
		Host:     GetEnv("DB_HOST", "localhost"),
		Port:     GetEnv("DB_PORT", "5432"),
		User:     GetEnv("DB_USER", "postgres"),
		Password: GetEnv("DB_PASSWORD", "123456"),
		DBName:   GetEnv("DB_NAME", "reaction"),
	}

	jwtDuration, err := time.ParseDuration(GetEnv("JWT_TOKEN_DURATION", "24h"))
	if err != nil {
		jwtDuration = 24 * time.Hour
	}

	cfg.JWT = JWTConfig{
		SecretKey:     GetEnv("JWT_SECRET_KEY", "very-very-secret-key"),
		TokenDuration: jwtDuration,
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
