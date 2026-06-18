package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/config"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/httpapi"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/push"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/sms"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "mobilechat-server ", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("database error: %v", err)
	}
	defer db.Close()

	if cfg.RunMigrationsOnStart {
		if err := db.RunMigrations(ctx, "migrations"); err != nil {
			logger.Fatalf("migration error: %v", err)
		}
	} else {
		logger.Println("startup migrations disabled; run the migration job before starting/updating production")
	}

	repo := storage.NewRepository(db.Pool)
	if err := repo.SyncAdminPhoneAllowlist(ctx, cfg.SuperAdminPhones, cfg.PlatformAdminPhones); err != nil {
		logger.Fatalf("admin allowlist sync error: %v", err)
	}
	if len(cfg.SuperAdminPhones) > 0 || len(cfg.PlatformAdminPhones) > 0 {
		logger.Printf("admin allowlist synced: super_admin=%d platform_admin=%d", len(cfg.SuperAdminPhones), len(cfg.PlatformAdminPhones))
	}

	notifier := &push.FCMNotifier{
		ProjectID:   cfg.FCMProjectID,
		ClientEmail: cfg.FCMClientEmail,
		PrivateKey:  cfg.FCMPrivateKey,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
	if !notifier.Enabled() {
		logger.Println("push notifications disabled: FCM env vars are not configured")
	}
	svc := service.New(repo, service.Config{
		JWTSecret:      cfg.JWTSecret,
		AccessTokenTTL: cfg.AccessTokenTTL,
		BCryptCost:     cfg.BCryptCost,
	}, notifier)
	svc.SetContentModerator(moderation.NewCompositeModerator(moderation.OpenAIConfig{
		Enabled:    cfg.ContentModerationEnabled,
		APIKey:     cfg.OpenAIModerationAPIKey,
		Model:      cfg.OpenAIModerationModel,
		Endpoint:   cfg.OpenAIModerationEndpoint,
		FailClosed: cfg.ModerationFailClosed,
		Timeout:    5 * time.Second,
	}))
	if cfg.ContentModerationEnabled && cfg.OpenAIModerationAPIKey != "" {
		logger.Printf("content moderation enabled with OpenAI model %s", cfg.OpenAIModerationModel)
	} else if cfg.ContentModerationEnabled {
		logger.Println("content moderation enabled with local rules only: OPENAI_API_KEY is not configured")
	} else {
		logger.Println("content moderation disabled")
	}

	var smsSender sms.Sender = sms.DevSender{Logger: logger}
	if cfg.SMSProvider != "dev" {
		smsSender = sms.DisabledSender{}
	}
	phoneAuth := service.NewPhoneAuth(repo, service.PhoneAuthConfig{
		JWTSecret:           cfg.JWTSecret,
		AccessTokenTTL:      cfg.AccessTokenTTL,
		Environment:         cfg.Environment,
		TestAuthEnabled:     cfg.TestAuthEnabled,
		TestAuthPhone:       cfg.TestAuthPhone,
		TestAuthCode:        cfg.TestAuthCode,
		TestAuthDisplayName: cfg.TestAuthDisplayName,
	}, smsSender)

	handler := httpapi.New(svc, phoneAuth, logger, cfg.AllowedOrigins)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Printf("listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	logger.Println("shutdown started")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			logger.Printf("forced close failed: %v", closeErr)
		}
	}
	logger.Println("shutdown completed")
}
