package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tn00869679/QuizForge_Backend/internal/config"
	"github.com/tn00869679/QuizForge_Backend/internal/db"
	"github.com/tn00869679/QuizForge_Backend/internal/handler"
	"github.com/tn00869679/QuizForge_Backend/internal/middleware"
	"golang.org/x/time/rate"
)

func main() {
	cfg := config.Load()

	slog.Info("running migrations")
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("db pool failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.CORS(cfg.CORSOrigins),
		middleware.RateLimit(rate.Limit(10), 20),
	)

	r.GET("/health", func(c *gin.Context) {
		handler.Ok(c, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	handlers := handler.New(handler.Deps{Pool: pool, Cfg: cfg})
	handlers.RegisterRoutes(
		v1,
		middleware.UserStub(),
		middleware.RequireUser(),
		middleware.RequireAdmin(cfg.AdminToken),
		middleware.RateLimit(rate.Limit(1), 5),
	)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	slog.Info("server starting", "port", cfg.Port)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
