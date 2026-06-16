package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/config"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/database"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/handler"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/kafka"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/repository"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/service"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/telemetry"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// ── Load config ──────────────────────────────────────────────
	cfg := config.Load()

	// ── Logger ───────────────────────────────────────────────────
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync()

	// ── OpenTelemetry Tracing ─────────────────────────────────────
	shutdown, err := telemetry.InitTracer(cfg.JaegerEndpoint, "url-service")
	if err != nil {
		logger.Fatal("failed to init tracer", zap.Error(err))
	}
	defer shutdown(context.Background())

	// ── Database ──────────────────────────────────────────────────
	db, err := database.NewPostgres(cfg)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	// ── Run Migrations ────────────────────────────────────────────
	if err := database.RunMigrations(db); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// ── Kafka Producer ────────────────────────────────────────────
	producer := kafka.NewProducer(cfg.KafkaBrokers, logger)
	defer producer.Close()

	// ── Dependency Injection ──────────────────────────────────────
	urlRepo := repository.NewURLRepository(db, logger)
	outboxRepo := repository.NewOutboxRepository(db, logger)
	urlSvc := service.NewURLService(urlRepo, outboxRepo, producer, logger)
	urlHandler := handler.NewURLHandler(urlSvc, logger)

	// ── Start Outbox Poller (Transactional Outbox Pattern) ────────
	outboxPoller := kafka.NewOutboxPoller(outboxRepo, producer, logger)
	go outboxPoller.Start(context.Background())

	// ── HTTP Router ───────────────────────────────────────────────
	router := setupRouter(urlHandler, logger)

	// ── HTTP Server ───────────────────────────────────────────────
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful Shutdown ─────────────────────────────────────────
	go func() {
		logger.Info("URL Service starting", zap.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced shutdown", zap.Error(err))
	}
	logger.Info("server exited cleanly")
}

func setupRouter(h *handler.URLHandler, logger *zap.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware(logger))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "url-service"})
	})

	// Metrics endpoint
	router.GET("/metrics", handler.MetricsHandler())

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		urls := v1.Group("/urls")
		{
			urls.POST("", h.CreateURL)           // Create short URL
			urls.GET("/:id", h.GetURL)            // Get URL by ID
			urls.PUT("/:id", h.UpdateURL)         // Update URL
			urls.DELETE("/:id", h.DeleteURL)      // Delete URL
			urls.GET("", h.ListURLs)              // List user's URLs
		}
	}

	return router
}

func loggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}
