package logger

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func New(level, format string) *logrus.Logger {
	logger := logrus.New()

	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		fmt.Printf("Failed to parse log level\n")
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)

	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			ForceColors:     true,
		})
	}

	logger.SetOutput(os.Stdout)

	return logger
}
