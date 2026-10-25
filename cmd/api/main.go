package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/quan/task-management-api/configs"
	"github.com/quan/task-management-api/internal/handlers"
	"github.com/quan/task-management-api/internal/models"
	"github.com/quan/task-management-api/internal/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := configs.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.Task{}, &models.Comment{}); err != nil {
		log.Fatalf("apply schema: %v", err)
	}
	if cfg.AdminEmail != "" {
		var count int64
		cfg.AdminEmail = strings.ToLower(strings.TrimSpace(cfg.AdminEmail))
		if err := db.Model(&models.User{}).Where("email = ?", cfg.AdminEmail).Count(&count).Error; err != nil {
			log.Fatalf("check bootstrap admin: %v", err)
		}
		if count == 0 {
			user, err := services.New(db).Register(cfg.AdminName, cfg.AdminEmail, cfg.AdminPassword)
			if err != nil {
				log.Fatalf("create bootstrap admin: %v", err)
			}
			if err := db.Model(&user).Update("role", "admin").Error; err != nil {
				log.Fatalf("promote bootstrap admin: %v", err)
			}
		}
	}
	var cache *redis.Client
	if cfg.RedisURL != "" {
		options, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("parse REDIS_URL: %v", err)
		}
		cache = redis.NewClient(options)
		if err := cache.Ping(context.Background()).Err(); err != nil {
			log.Fatalf("connect Redis: %v", err)
		}
	}
	server := &http.Server{Addr: ":" + cfg.Port, Handler: (handlers.API{Config: cfg, DB: db, Cache: cache}).Router(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("API listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
	}
	if cache != nil {
		_ = cache.Close()
	}
}
