package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/cache"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/circuitbreaker"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/config"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/database"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/handler"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/kafka"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/repository"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync()

	// Database
	db, err := database.NewPostgres(cfg)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}

	// Redis Cache
	redisCache := cache.NewRedisCache(cfg.RedisHost, cfg.RedisPort, logger)

	// Circuit Breaker for DB calls
	cb := circuitbreaker.New("postgres", circuitbreaker.Config{
		MaxFailures:     5,
		ResetTimeout:    30 * time.Second,
		HalfOpenMaxReqs: 3,
	}, logger)

	// Kafka Producer (for click events — non-blocking async)
	producer := kafka.NewAsyncProducer(cfg.KafkaBrokers, logger)
	defer producer.Close()

	// Dependency injection
	urlRepo := repository.NewURLRepository(db, logger)
	redirectSvc := service.NewRedirectService(urlRepo, redisCache, cb, producer, logger)
	redirectHandler := handler.NewRedirectHandler(redirectSvc, logger)

	// Router
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "redirect-service"})
	})

	// THE HOT PATH — this route handles 115,000+ req/sec
	router.GET("/:short_code", redirectHandler.Redirect)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,  // tight timeouts for hot path
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info("Redirect Service starting", zap.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	logger.Info("redirect service exited")
}
