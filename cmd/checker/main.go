package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"balance_checker/internal/app/service"
	dex_client "balance_checker/internal/client"
	"balance_checker/internal/infrastructure/configloader"
	clientprovider "balance_checker/internal/infrastructure/network/client"
	networkdefinition "balance_checker/internal/infrastructure/network/definition"
	"balance_checker/internal/infrastructure/restapi"
	"balance_checker/internal/infrastructure/tokenloader"
	"balance_checker/internal/infrastructure/walletloader"
	"balance_checker/internal/pkg/logger"

	slogzap "github.com/samber/slog-zap/v2"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

const (
	defaultConfigPath = "config/config.yml"
	defaultTokensDir  = "data/tokens/"
)

func main() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempZapLogger, errTempLog := zap.NewDevelopment()
	if errTempLog != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to initialize temporary zapLogger: %v\n", errTempLog)
		os.Exit(1)
	}

	cfg, err := configloader.Load(defaultConfigPath)
	if err != nil {
		tempZapLogger.Fatal("Не удалось загрузить конфигурацию", zap.String("файл", "config.yml"), zap.Error(err))
	}

	zapLogger, errLog := zap.NewDevelopment()
	if errLog != nil {
		// Если основной логгер не удалось создать, используем временный для фатальной ошибки
		tempZapLogger.Fatal("Не удалось инициализировать основной zapLogger", zap.Error(errLog))
	}
	defer zapLogger.Sync()

	if zapLogger != nil {
		defer zapLogger.Sync()
	}

	if zapLogger == nil {
		if tempZapLogger != nil {
			tempZapLogger.Fatal("zapLogger is nil before creating slogHandler via samber/slog-zap. This should not happen.")
		} else {
			fmt.Fprintln(os.Stderr, "CRITICAL: zapLogger and tempZapLogger are nil before slog Handler creation.")
			os.Exit(1)
		}
	}

	slogLevel := slog.LevelDebug
	if core := zapLogger.Core(); core != nil {
	}

	slogHandlerOptions := slogzap.Option{
		Level:  slogLevel,
		Logger: zapLogger,
	}
	slogHandler := slogHandlerOptions.NewZapHandler()

	stdSlogLogger := slog.New(slogHandler)
	slog.SetDefault(stdSlogLogger)

	logger.Info("Сервис проверки балансов запускается...")
	logger.Info("Конфигурация успешно загружена.")

	if cfg.Logging.Level == "debug" {
		logger.Debug("Debug mode enabled")
	}

	logger.Info("Установлен лимит параллельных горутин", "количество", cfg.Performance.MaxConcurrentRoutines)

	appLogger := logger.NewSlogAdapter()
	appLogger.Info("Логгер port.Logger (slogAdapter) инициализирован")

	walletProvider := walletloader.NewWalletFileLoader(appLogger.Info)
	logger.Info("WalletProvider инициализирован.")

	netDefProvider := networkdefinition.NewNetworkDefinitionProvider(appLogger, defaultTokensDir)

	tokenProvider := tokenloader.NewTokenLoader(appLogger.Info, appLogger.Warn)
	logger.Info("TokenProvider инициализирован.")

	clientProvider := clientprovider.NewEVMClientProvider(cfg, appLogger.Info, appLogger.Error)
	logger.Info("BlockchainClientProvider инициализирован.")

	dexScreenerRequestTimeout := time.Duration(cfg.DEXScreener.RequestTimeoutMillis) * time.Millisecond
	dexscreenerAPIClient := dex_client.NewDEXScreenerClient(
		cfg.DEXScreener.BaseURL,
		dexScreenerRequestTimeout,
		zapLogger.Named("DEXScreenerAPIClient"),
		cfg.TokenPriceSvc.MaxTokensPerBatchRequest,
	)
	logger.Info("DEXScreenerClient (из internal/client) успешно инициализирован.")

	tokenPriceService := service.NewTokenPriceService(
		tokenProvider,
		netDefProvider,
		dexscreenerAPIClient,
		appLogger,
		cfg,
	)
	logger.Info("TokenPriceService успешно инициализирован.")

	logger.Info("Запуск начальной загрузки и кеширования цен токенов...")
	if err := tokenPriceService.LoadAndCacheTokenPrices(context.Background()); err != nil {
		logger.Fatal("Не удалось загрузить и закешировать цены токенов", "ошибка", err)
	}
	logger.Info("Начальная загрузка и кеширование цен токенов успешно завершены.")

	portfolioService := service.NewPortfolioService(
		walletProvider,
		netDefProvider,
		tokenProvider,
		clientProvider,
		tokenPriceService,
		appLogger,
		cfg,
		cfg.Performance.MaxConcurrentRoutines,
	)
	logger.Info("PortfolioService успешно инициализирован.")

	logger.Info("Настройка HTTP API...")
	portfolioAPIHandler := restapi.NewPortfolioHandler(portfolioService, cfg, appLogger)

	ginRouter := restapi.SetupRouter(portfolioAPIHandler)

	ginRouter.StaticFile("/docs/swagger.yaml", "./docs/swagger.yaml")

	swaggerURL := ginSwagger.URL("/docs/swagger.yaml")
	ginRouter.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerURL))

	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	appLogger.Info("Запуск сервера", "address", serverAddr)

	srv := &http.Server{
		Addr:    serverAddr,
		Handler: ginRouter,
	}

	go func() {
		logger.Info("Запуск HTTP сервера", "адрес", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Не удалось запустить HTTP сервер", "ошибка", err)
		}
	}()

	logger.Info("Приложение работает. HTTP API доступен. Нажмите Ctrl+C для завершения.")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	logger.Info("Получен сигнал завершения. Завершение работы HTTP сервера...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Ошибка при Graceful Shutdown HTTP сервера", "ошибка", err)
	} else {
		logger.Info("HTTP сервер успешно остановлен.")
	}

	cancel()

	logger.Info("Сервис проверки балансов остановлен.")
}
