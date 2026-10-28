package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/config"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "mobilechat-migrate ", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("database error: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(ctx, "migrations"); err != nil {
		logger.Fatalf("migration error: %v", err)
	}
	logger.Println("migrations completed")
}
