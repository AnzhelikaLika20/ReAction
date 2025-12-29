package config

import (
	"fmt"
	"os"
	"strconv"
)

type AppConfig struct {
	Telegram TelegramConfig
	Logging  LoggingConfig
	Server   ServerConfig
	Kafka    KafkaConfig
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
	Broker        string `yaml:"broker" env:"KAFKA_BROKER" envSeparator:","`
	TopicMessages string `yaml:"topic_messages" env:"KAFKA_TOPIC_MESSAGES" envDefault:"telegram-messages"`
	TopicUpdates  string `yaml:"topic_updates" env:"KAFKA_TOPIC_UPDATES" envDefault:"telegram-updates"`
	GroupID       string `yaml:"group_id" env:"KAFKA_GROUP_ID" envDefault:"reaction-telegram"`
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
	if topic := os.Getenv("KAFKA_TOPIC_MESSAGES"); topic != "" {
		cfg.Kafka.TopicMessages = topic
	}
	if topic := os.Getenv("KAFKA_TOPIC_UPDATES"); topic != "" {
		cfg.Kafka.TopicUpdates = topic
	}
	cfg.Kafka.GroupID = "reaction-telegram"

	return cfg, nil
}

func MustLoad() *AppConfig {
	cfg, err := Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		fmt.Println("\nPlease set the following environment variables:")
		fmt.Println("  TELEGRAM_API_ID     - Your Telegram API ID")
		fmt.Println("  TELEGRAM_API_HASH   - Your Telegram API Hash")
		fmt.Println("\nYou can create a .env file with these variables")
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
