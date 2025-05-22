package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof" // Blank import for pprof
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"balance_checker/api"
	"balance_checker/internal/client"
	"balance_checker/internal/config"
	"balance_checker/internal/repository"
	"balance_checker/internal/service"
	"balance_checker/internal/utils"
	"balance_checker/pkg/blockchain"
	"balance_checker/pkg/metrics"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

func main() {
	// Initialize logger (using logrus for now as per existing config, but can switch to zap native)
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	// Default level, will be updated by config
	log.SetLevel(logrus.InfoLevel)

	// Convert logrus to zap for context where zap is expected (like our services)
	// For a more robust solution, consider a unified logging setup from the start.
	zapLogger, err := zap.NewProduction() // Or NewDevelopment for more verbose logs
	if err != nil {
		log.Fatalf("Failed to initialize zap logger: %v", err)
	}
	defer zapLogger.Sync() // flushes buffer, if any

	// Use zapslog for slog compatibility if needed elsewhere, or directly use zapLogger
	slogHandler := zapslog.NewHandler(zapLogger.Core(), &zapslog.HandlerOptions{})
	stdLogger := slog.New(slogHandler)
	slog.SetDefault(stdLogger)

	// Load configuration
	cfgPath := utils.GetEnv("CONFIG_PATH", "config/config.yaml")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Update log level from config
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		log.Warnf("Invalid log level in config: %s. Defaulting to Info.", cfg.Logging.Level)
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	if cfg.Logging.File != "" {
		file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(file)
		} else {
			log.Infof("Failed to log to file, using default stdout: %v", err)
		}
	}

	zapLogger.Info("Configuration loaded", zap.String("path", cfgPath))

	// Initialize Prometheus metrics
	metrics.MustRegisterMetrics()

	// Initialize blockchain clients provider
	provider := blockchain.NewEVMClientProvider(cfg, zapLogger)

	// Initialize CoinGecko client (Marked as deprecated or for removal)
	// coinGeckoRequestTimeout := time.Duration(cfg.CoinGecko.RequestTimeoutMillis) * time.Millisecond
	// coinGeckoClient := client.NewCoinGeckoClient(cfg.CoinGecko.BaseURL, cfg.CoinGecko.ApiKey, coinGeckoRequestTimeout, zapLogger)
	// zapLogger.Info("CoinGecko client initialized (deprecated)")

	// Initialize DEXScreener client
	dexScreenerRequestTimeout := time.Duration(cfg.DEXScreener.RequestTimeoutMillis) * time.Millisecond
	dexScreenerClient := client.NewDEXScreenerClient(
		cfg.DEXScreener.BaseURL,
		dexScreenerRequestTimeout,
		zapLogger,
		cfg.TokenPriceSvc.MaxTokensPerBatchRequest, // Use this from TokenPriceSvc config as it's specific to how we batch for price fetching
	)
	zapLogger.Info("DEXScreener client initialized")

	// Initialize TokenPriceService with DEXScreenerClient
	tokenPriceService := service.NewTokenPriceService(zapLogger, cfg, dexScreenerClient)
	zapLogger.Info("TokenPriceService initialized with DEXScreener client")

	// Start caching token prices in the background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Context with timeout for initial price load
		defer cancel()
		if err := tokenPriceService.LoadAndCacheTokenPrices(ctx); err != nil {
			zapLogger.Error("Failed to perform initial load and cache of token prices", zap.Error(err))
		} else {
			zapLogger.Info("Initial token price loading and caching completed.")
		}
	}()

	// Initialize services
	var portfolioRepo repository.PortfolioRepository
	if cfg.PortfolioService.UseMock {
		// portfolioRepo = repository.NewMockPortfolioRepository() // If you have a mock implementation
		zapLogger.Info("Using MockPortfolioRepository")
		// For now, let's panic if mock is true but no mock repo is set up, or use a default real one.
		// This part needs to be correctly implemented if mocks are genuinely used.
		portfolioRepo = repository.NewInMemoryPortfolioRepository() // Placeholder
	} else {
		portfolioRepo = repository.NewInMemoryPortfolioRepository() // Or your persistent repository
		zapLogger.Info("Using InMemoryPortfolioRepository")
	}

	portfolioSvc := service.NewPortfolioService(portfolioRepo, provider, tokenPriceService, cfg, zapLogger)
	zapLogger.Info("PortfolioService initialized")

	// Initialize Gin router
	router := gin.New()

	// Setup CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true // Adjust for production
	corsConfig.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	router.Use(cors.New(corsConfig))

	// Custom logging middleware using zap
	router.Use(utils.ZapLoggerMiddleware(zapLogger))
	router.Use(gin.Recovery())

	// Setup routes
	api.RegisterPortfolioRoutes(router, portfolioSvc, cfg, zapLogger)

	// Swagger documentation if enabled
	if cfg.Swagger.Enabled {
		api.RegisterSwaggerRoutes(router, cfg.Swagger.Path)
		zapLogger.Info("Swagger UI enabled", zap.String("path", cfg.Swagger.Path+"/index.html"))
	}

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	zapLogger.Info("Prometheus metrics endpoint enabled", zap.String("path", "/metrics"))

	// Pprof endpoints (for debugging performance issues)
	// Make sure to protect these in a production environment
	pprofRouter := router.Group("/debug/pprof")
	{
		pprofRouter.GET("/", gin.WrapF(pprof.Index))
		pprofRouter.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofRouter.GET("/profile", gin.WrapF(pprof.Profile))
		pprofRouter.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofRouter.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofRouter.GET("/trace", gin.WrapF(pprof.Trace))
		pprofRouter.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
		pprofRouter.GET("/block", gin.WrapH(pprof.Handler("block")))
		pprofRouter.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		pprofRouter.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		pprofRouter.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		pprofRouter.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	}
	zapLogger.Info("Pprof endpoints enabled under /debug/pprof")

	// Start server
	srv := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		zapLogger.Info(fmt.Sprintf("Server starting on port %s", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zapLogger.Info("Shutting down server...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		zapLogger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	zapLogger.Info("Server exiting")
}
