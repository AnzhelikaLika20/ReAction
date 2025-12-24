package main

import (
	"fmt"
	"ReAction/internal/config"
	"ReAction/internal/api"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("Note: No .env file found")
	}
	
	cfg := config.MustLoad()
	api.RunHTTPServer(*cfg)
}