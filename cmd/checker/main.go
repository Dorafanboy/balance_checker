package main

import (
	"context"
	"fmt"
	"log/slog" // ИМПОРТ slog
	"net/http" // Добавлен для http.Server и Graceful Shutdown
	"os"
	"os/signal"
	"syscall"
	"time" // Добавлен для Graceful Shutdown

	// "log" // Стандартный log, возможно, не нужен, если используется кастомный logger.Fatal

	// "time" // Константа defaultConnectionTimeout больше не используется здесь

	"balance_checker/internal/app/service"
	dex_client "balance_checker/internal/client"           // ИЗМЕНЕНО: Прямой импорт DEXScreener клиента
	"balance_checker/internal/infrastructure/configloader" // ADDED: CoinGecko client package

	// ИСПРАВЛЕНО: правильный путь к httpclient
	clientprovider "balance_checker/internal/infrastructure/network/client"
	networkdefinition "balance_checker/internal/infrastructure/network/definition"
	"balance_checker/internal/infrastructure/restapi" // Добавлен для API
	"balance_checker/internal/infrastructure/tokenloader"
	"balance_checker/internal/infrastructure/walletloader"
	"balance_checker/internal/pkg/logger"

	// "balance_checker/internal/pkg/utils" // Проверить, используется ли. Если нет - удалить.

	slogzap "github.com/samber/slog-zap/v2" // ДОБАВЛЕНО: для slog-zap адаптера
	"go.uber.org/zap"                       // ИМПОРТ zap

	// "go.uber.org/zap/exp/zapslog" // ИМПОРТ zapslog // Комментируем прямой импорт // УДАЛЕНО: exp/zapslog больше не нужен
	// expzapslog "go.uber.org/zap/exp/zapslog" // ИМПОРТ zapslog с псевдонимом // УДАЛЕНО: exp/zapslog больше не нужен
	"github.com/gin-gonic/gin"

	// Swagger imports
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
)

// Константы defaultXXXPath больше не нужны, т.к. пути берутся из конфига
// const (
// 	defaultConfigPath        = "config/config.yml"
// 	defaultWalletsPath       = "data/wallets.txt"
// 	defaultTokensDir         = "data/tokens"
// 	defaultConnectionTimeout = 10 * time.Second
// )

func main() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Предварительная инициализация базового логгера для самой ранней загрузки конфига
	tempZapLogger, errTempLog := zap.NewDevelopment()
	if errTempLog != nil {
		// Если даже временный логгер не создать, пишем в stderr и выходим
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to initialize temporary zapLogger: %v\n", errTempLog)
		os.Exit(1)
	}

	// Загрузка конфигурации
	cfg, err := configloader.Load("config/config.yml")
	if err != nil {
		tempZapLogger.Fatal("Не удалось загрузить конфигурацию", zap.String("файл", "config.yml"), zap.Error(err))
	}

	// Инициализация основного zapLogger на основе конфига
	// Уровень логирования из cfg.Logging.Level нужно будет преобразовать в zap уровень или использовать стандартный
	// Пока используем Production, но в идеале настроить уровень из cfg.
	zapLogger, errLog := zap.NewDevelopment()
	if errLog != nil {
		// Если основной логгер не удалось создать, используем временный для фатальной ошибки
		tempZapLogger.Fatal("Не удалось инициализировать основной zapLogger", zap.Error(errLog))
	}
	// defer zapLogger.Sync() // Пока закомментируем, чтобы исключить его влияние на панику до строки 70

	// Пока оставим инициализацию zapLogger, но он не используется для slog.SetDefault
	if zapLogger != nil { // zapLogger все еще существует от NewDevelopment() вызова выше
		defer zapLogger.Sync()
	}

	// Используем samber/slog-zap адаптер
	if zapLogger == nil {
		// Если zapLogger по какой-то причине nil (хотя не должен быть после NewDevelopment)
		// Можно создать временный или запаниковать, здесь для примера паникуем.
		// Этот tempZapLogger должен быть инициализирован ранее.
		if tempZapLogger != nil {
			tempZapLogger.Fatal("zapLogger is nil before creating slogHandler via samber/slog-zap. This should not happen.")
		} else {
			// Крайний случай, если и tempZapLogger недоступен
			fmt.Fprintln(os.Stderr, "CRITICAL: zapLogger and tempZapLogger are nil before slog Handler creation.")
			os.Exit(1)
		}
	}

	// Определяем уровень логирования для slog на основе уровня zapLogger
	// По умолчанию slog.LevelDebug, если не удается определить точнее
	slogLevel := slog.LevelDebug
	if core := zapLogger.Core(); core != nil {
		// Пытаемся получить уровень из zap.Core.
		// Это не прямой способ, но для примера. zap.AtomicLevel может быть полезнее.
		// Для простоты пока оставим slog.LevelDebug или настроим явно.
		// zapLevel := core.Enabled(zapcore.DebugLevel) // Проверка, включен ли Debug
		// Можно сделать более сложную логику для маппинга zap уровней в slog уровни
	}

	// Создаем slog.Handler с помощью samber/slog-zap
	// Передаем существующий zapLogger
	slogHandlerOptions := slogzap.Option{
		Level:  slogLevel, // Используем определенный ранее slogLevel
		Logger: zapLogger,
		// Можно добавить другие опции из slog.HandlerOptions если нужно, например:
		// AddSource: true,
		// ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		//    // ... custom attribute replacement ...
		//    return a
		// },
	}
	slogHandler := slogHandlerOptions.NewZapHandler()

	stdSlogLogger := slog.New(slogHandler)
	slog.SetDefault(stdSlogLogger) // Устанавливаем глобальный slog логгер

	logger.Info("Сервис проверки балансов запускается...") // Теперь это будет использовать slog, который пишет в zap
	logger.Info("Конфигурация успешно загружена.")

	if cfg.Logging.Level == "debug" {
		logger.Debug("Debug mode enabled")
	}

	logger.Info("Установлен лимит параллельных горутин", "количество", cfg.Performance.MaxConcurrentRoutines)

	// Создание адаптера логгера для тех частей, которые ожидают port.Logger (он будет использовать глобальный slog)
	appLogger := logger.NewSlogAdapter()
	appLogger.Info("Логгер port.Logger (slogAdapter) инициализирован")

	// Инициализация WalletProvider
	// cfg.WalletFilePath должен быть определен в структуре Config (например, Files.Wallets)
	// Предположим, что путь к файлу кошельков теперь в cfg.Files.Wallets
	// Если такого поля нет, нужно добавить его в configloader.Config и config.yml
	// Для примера, используем cfg.Server.WalletsPath (нужно будет добавить в ServerConfig или создать FilesConfig)
	// Пока что оставим как было, но это нужно будет согласовать со структурой Config
	walletProvider := walletloader.NewWalletFileLoader(appLogger.Info) // Используем конструктор и передаем appLogger.Info, cfg.WalletFilePath удален
	// loadedWallets больше не загружаются здесь, это делает PortfolioService
	logger.Info("WalletProvider инициализирован.")

	// Инициализация NetworkProvider
	// Передаем путь к директории токенов вместо cfg.Networks
	// ПУТЬ К ДИРЕКТОРИИ ТОКЕНОВ: "data/tokens/" - можно вынести в константу или cfg, если потребуется
	netDefProvider := networkdefinition.NewNetworkDefinitionProvider(appLogger, "data/tokens/")
	// logger.Info("NetworkDefinitionProvider инициализирован.") // Логирование теперь внутри конструктора

	// Инициализация TokenProvider
	// cfg.TokenDirectoryPath должен быть определен в структуре Config
	// Пока что оставим как было, но это нужно будет согласовать со структурой Config
	tokenProvider := tokenloader.NewTokenLoader(appLogger.Info, appLogger.Warn) // Используем конструктор, cfg.TokenDirectoryPath удален
	logger.Info("TokenProvider инициализирован.")

	// Инициализация Blockchain Client Provider
	clientProvider := clientprovider.NewEVMClientProvider(cfg, appLogger.Info, appLogger.Error)
	logger.Info("BlockchainClientProvider инициализирован.")

	// Инициализация DEXScreenerClient
	// Используем импортированный dex_client.NewDEXScreenerClient
	dexScreenerRequestTimeout := time.Duration(cfg.DEXScreener.RequestTimeoutMillis) * time.Millisecond
	dexscreenerAPIClient := dex_client.NewDEXScreenerClient(
		cfg.DEXScreener.BaseURL,
		dexScreenerRequestTimeout,
		zapLogger.Named("DEXScreenerAPIClient"),    // ПЕРЕДАЕМ zapLogger
		cfg.TokenPriceSvc.MaxTokensPerBatchRequest, // Используем MaxTokensPerBatchRequest из конфига TokenPriceSvc
	)
	logger.Info("DEXScreenerClient (из internal/client) успешно инициализирован.")

	// Инициализация TokenPriceService
	tokenPriceService := service.NewTokenPriceService(
		tokenProvider,
		netDefProvider,
		dexscreenerAPIClient, // <--- ПЕРЕДАЕМ НОВЫЙ КЛИЕНТ
		appLogger,
		cfg,
	)
	logger.Info("TokenPriceService успешно инициализирован.")

	// Загрузка и кеширование цен на токены
	logger.Info("Запуск начальной загрузки и кеширования цен токенов...")
	if err := tokenPriceService.LoadAndCacheTokenPrices(context.Background()); err != nil { // Используем context.Background()
		// Вместо Fatal можно сделать Warn и продолжить работу без цен, или с частично загруженными
		// Пока что оставляем Fatal, т.к. цены важны для новой функциональности
		logger.Fatal("Не удалось загрузить и закешировать цены токенов", "ошибка", err)
	}
	logger.Info("Начальная загрузка и кеширование цен токенов успешно завершены.")

	// Инициализация PortfolioService
	portfolioService := service.NewPortfolioService(
		walletProvider,
		netDefProvider, // Передаем NetworkDefinitionProvider
		tokenProvider,
		clientProvider,    // Передаем BlockchainClientProvider
		tokenPriceService, // ДОБАВЛЕНО: Передаем TokenPriceService
		appLogger,         // Передаем адаптер логгера
		cfg,               // Передаем конфигурацию
		cfg.Performance.MaxConcurrentRoutines,
	)
	logger.Info("PortfolioService успешно инициализирован.")
	// Убираем или комментируем старую логику консольного вывода, так как теперь будет API
	/*
		logger.Info("Настройка приложения завершена. Запуск получения портфелей...")

		portfolios, portfolioErrors := portfolioService.FetchAllWalletsPortfolio(ctx, cfg.TrackedNetworkIdentifiers)

		if len(portfolioErrors) > 0 {
			logger.Warn("Во время получения портфелей возникли ошибки:", "количество_ошибок", len(portfolioErrors))
			for i, e := range portfolioErrors {
				logger.Warn("Ошибка", "индекс", i+1, "кошелек", e.WalletAddress, "сеть", e.NetworkName, "токен", e.TokenSymbol, "сообщение", e.Message)
			}
		}
		failedWallets := portfolioService.GetFailedWallets()
		logger.Info("Обработка портфелей завершена", "получено_портфелей", len(portfolios), "кошельков_с_ошибками", len(failedWallets))

		for i, p := range portfolios {
			logger.Info("Портфель для кошелька",
				"индекс_кошелька", i+1,
				"адрес_кошелька", p.WalletAddress,
				"количество_балансов", len(p.Balances),
				"количество_ошибок_в_портфеле", p.ErrorCount,
			)
			for _, balance := range p.Balances {
				logger.Info("  Баланс",
					"сеть", balance.NetworkName,
					"токен", balance.TokenSymbol,
					"количество", balance.Amount,
					"отформатировано", balance.FormattedBalance,
					"адрес_контракта", balance.TokenAddress,
				)
			}
			for _, portfolioErr := range p.Errors {
				logger.Warn("    Ошибка в портфеле", "сеть", portfolioErr.NetworkName, "токен", portfolioErr.TokenSymbol, "сообщение", portfolioErr.Message)
			}
		}
	*/

	// Настройка и запуск HTTP сервера
	logger.Info("Настройка HTTP API...")
	portfolioAPIHandler := restapi.NewPortfolioHandler(portfolioService, cfg, appLogger)

	// Настройка маршрутов Gin
	ginRouter := gin.Default()

	// Swagger документация (если используется)
	// docs.SwaggerInfo.BasePath = "/api/v1"
	// ginRouter.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiV1 := ginRouter.Group("/api/v1")
	{
		apiV1.GET("/portfolios", portfolioAPIHandler.GetPortfoliosHandler)
		apiV1.GET("/portfolios/failed", portfolioAPIHandler.GetFailedWalletsHandler)

		// Новый маршрут для получения портфеля по конкретному адресу кошелька
		// GET /api/v1/portfolios/{walletAddress}
		// Query params: ?network=name1&network=name2 или ?networks=name1,name2
		apiV1.GET("/portfolios/:walletAddress", portfolioAPIHandler.GetSingleWalletPortfolioHandler)

		// Новый маршрут для получения портфеля по конкретной сети конкретного кошелька
		// GET /api/v1/portfolios/{walletAddress}/networks/{networkIdentifier}
		apiV1.GET("/portfolios/:walletAddress/networks/:networkIdentifier", portfolioAPIHandler.GetSingleWalletNetworkPortfolioHandler)

		// Заглушки для предложенных, но не реализуемых пока эндпоинтов (если они были в SetupRouter)
		// apiV1.GET("/networks", networkHandler.ListNetworksHandler) // Пример
		// apiV1.GET("/networks/:networkIdentifier/tokens", tokenHandler.ListTokensInNetworkHandler) // Пример
		// apiV1.GET("/networks/:networkIdentifier/tokens/:tokenAddress", tokenHandler.GetTokenDetailsHandler) // Пример
	}

	// Swagger UI маршрут
	// Вместо docs.SwaggerInfo.BasePath (которое используется с swag init), мы просто указываем URL к нашему swagger.yaml
	// Предполагается, что swagger.yaml будет доступен по /docs/swagger.yaml (статический файл)
	// Сначала настроим отдачу статического файла:
	ginRouter.StaticFile("/docs/swagger.yaml", "./docs/swagger.yaml")

	swaggerURL := ginSwagger.URL("/docs/swagger.yaml") // Указываем Gin Swagger, где найти спецификацию
	ginRouter.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerURL))

	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	appLogger.Info("Запуск сервера", "address", serverAddr)

	srv := &http.Server{
		Addr:    serverAddr, // Используем сформированный serverAddr
		Handler: ginRouter,
	}

	go func() {
		logger.Info("Запуск HTTP сервера", "адрес", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Не удалось запустить HTTP сервер", "ошибка", err)
		}
	}()

	logger.Info("Приложение работает. HTTP API доступен. Нажмите Ctrl+C для завершения.")

	// Ожидание сигнала завершения (например, Ctrl+C)
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

	cancel() // Отменяем основной контекст приложения

	logger.Info("Сервис проверки балансов остановлен.")
}
