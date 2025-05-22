package restapi

import (
	"net/http"
	"strings"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/configloader"

	"github.com/gin-gonic/gin"
)

// APIPortfolioResponse определяет структуру ответа для эндпоинта портфелей.
type APIPortfolioResponse struct {
	Data struct {
		Portfolios         []entity.WalletPortfolio `json:"portfolios"`
		GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
	} `json:"data"`
	Errors        []entity.PortfolioError `json:"errors,omitempty"`
	StatusMessage string                  `json:"status_message"`
}

// APIPortfolioData содержит основные данные ответа API.
// Это позволяет легко добавлять метаданные или другую информацию верхнего уровня в будущем.
type APIPortfolioData struct {
	WalletsPortfolio   []entity.WalletPortfolio `json:"wallets_portfolio"`
	GrandTotalValueUSD *float64                 `json:"grand_total_value_usd,omitempty"` // Общая стоимость всех активов
}

// PortfolioHandler обрабатывает HTTP запросы, связанные с портфелями.
type PortfolioHandler struct {
	portfolioService port.PortfolioService
	cfg              *configloader.Config
	logger           port.Logger
}

// NewPortfolioHandler создает новый экземпляр PortfolioHandler.
func NewPortfolioHandler(ps port.PortfolioService, cfg *configloader.Config, logger port.Logger) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioService: ps,
		cfg:              cfg,
		logger:           logger,
	}
}

// GetPortfoliosHandler обрабатывает запрос на получение всех портфелей.
func (h *PortfolioHandler) GetPortfoliosHandler(c *gin.Context) {
	ctx := c.Request.Context() // Используем контекст из Gin запроса

	var finalNetworkIdentifiers []string
	networksQueryParam := c.Query("networks") // Получаем ?networks=eth,bsc

	if networksQueryParam != "" {
		// Если параметр networks предоставлен, используем его
		rawIdentifiers := strings.Split(networksQueryParam, ",")
		for _, id := range rawIdentifiers {
			trimmedID := strings.TrimSpace(id)
			if trimmedID != "" {
				finalNetworkIdentifiers = append(finalNetworkIdentifiers, trimmedID)
			}
		}
		h.logger.Debug("GetPortfoliosHandler: using 'networks' query parameter for filtering", "networks", finalNetworkIdentifiers)
	} else {
		// Иначе (параметр networks не предоставлен или пуст), используем все сети из конфигурации
		if h.cfg != nil && len(h.cfg.Networks) > 0 {
			activeNetworkIdentifiers := make([]string, 0, len(h.cfg.Networks))
			for _, netCfg := range h.cfg.Networks {
				if netCfg.Name != "" { // Используем имя сети как идентификатор
					activeNetworkIdentifiers = append(activeNetworkIdentifiers, netCfg.Name)
				}
				// Если Name пусто, пропускаем эту конфигурацию сети, так как она не может быть идентифицирована
			}
			finalNetworkIdentifiers = activeNetworkIdentifiers
		}
		h.logger.Debug("GetPortfoliosHandler: 'networks' query parameter not provided, using all configured networks", "configured_networks_count", len(finalNetworkIdentifiers))
	}

	// Если finalNetworkIdentifiers пуст (например, параметр networks был пуст или невалиден, ИЛИ конфигурация сетей пуста),
	// сервис должен это обработать (например, вернуть пустой результат или ошибку, если это не ожидаемое состояние).

	portfolios, serviceErrors := h.portfolioService.FetchAllWalletsPortfolio(ctx, finalNetworkIdentifiers)

	// Рассчитываем GrandTotalValueUSD
	var grandTotal float64
	for _, p := range portfolios {
		grandTotal += p.TotalValueUSD // Просто суммируем, так как это float64
	}

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         portfolios,
			GrandTotalValueUSD: grandTotal, // Присваиваем рассчитанное значение
		},
		Errors: serviceErrors,
	}

	if len(response.Errors) > 0 && len(portfolios) == 0 {
		response.StatusMessage = "Failed to retrieve any portfolios due to service errors."
		// Можно рассмотреть изменение HTTP статус кода, если нет ни одного портфеля и есть ошибки.
		// Например, http.StatusPartialContent или оставить http.StatusOK, но с явным сообщением.
		// Пока оставляем OK, но с сообщением.
	} else if len(response.Errors) > 0 {
		response.StatusMessage = "Portfolios retrieved. Some wallets or tokens may have encountered errors."
	} else if len(portfolios) == 0 {
		// Это может случиться, если нет кошельков в wallets.txt или нет активных сетей/токенов.
		response.StatusMessage = "No portfolio data found. Check wallet list and network/token configurations."
	} else {
		response.StatusMessage = "Portfolios retrieved successfully."
	}

	c.JSON(http.StatusOK, response)
}

// GetFailedWalletsHandler обрабатывает запрос на получение списка кошельков с ошибками.
// @Summary Get failed wallets
// @Description Retrieves a list of wallet addresses for which processing encountered errors.
// @Tags portfolios
// @Produce json
// @Success 200 {object} map[string]interface{} "json_object_with_failed_wallets_and_count"
// @Router /portfolios/failed [get]
func (h *PortfolioHandler) GetFailedWalletsHandler(c *gin.Context) {
	h.logger.Debug("Handler GetFailedWalletsHandler called")
	failedWallets := h.portfolioService.GetFailedWallets()
	if failedWallets == nil { // Обеспечиваем, чтобы всегда возвращался срез, а не nil
		failedWallets = make([]string, 0)
	}
	c.JSON(http.StatusOK, gin.H{
		"failed_wallets": failedWallets,
		"count":          len(failedWallets),
	})
}

// GetSingleWalletPortfolioHandler handles requests to get portfolio for a single wallet address.
// @Summary Get portfolio for a single wallet
// @Description Retrieves the portfolio for a single wallet address.
// @Tags portfolios
// @Produce json
// @Param walletAddress path string true "Wallet address"
// @Success 200 {object} APIPortfolioResponse "Portfolio retrieved successfully"
// @Failure 404 {object} gin.H "Wallet not found"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /portfolios/{walletAddress} [get]
func (h *PortfolioHandler) GetSingleWalletPortfolioHandler(c *gin.Context) {
	ctx := c.Request.Context()
	walletAddress := c.Param("walletAddress")

	// Извлечение query-параметра 'networks'
	// Ожидаем 'networks' как строку, разделенную запятыми (networks=name1,name2)
	var trackedNetworkNames []string
	networksSingleQuery := c.Query("networks") // Пытаемся получить как строку ?networks=eth,bsc
	if networksSingleQuery != "" {
		trackedNetworkNames = strings.Split(networksSingleQuery, ",")
		// Очистка от пробелов, если они есть
		for i, name := range trackedNetworkNames {
			trackedNetworkNames[i] = strings.TrimSpace(name)
		}
	}
	// Если networksSingleQuery пустой, trackedNetworkNames останется nil/empty,
	// сервис должен обработать это как "все сконфигурированные сети для данного кошелька".

	h.logger.Debug("Handler GetSingleWalletPortfolioHandler called", "walletAddress", walletAddress, "trackedNetworksQuery", trackedNetworkNames)

	portfolio, partialErrs, err := h.portfolioService.FetchSingleWalletPortfolioByAddress(ctx, walletAddress, trackedNetworkNames)

	if err != nil {
		// Проверяем, является ли ошибка ошибкой "не найдено"
		// Это упрощенная проверка; в идеале, сервис бы возвращал типизированные ошибки.
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.logger.Warn("Data not found for single wallet portfolio request", "walletAddress", walletAddress, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			h.logger.Error("Internal server error fetching single wallet portfolio", "walletAddress", walletAddress, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: " + err.Error()})
		}
		return
	}

	// Рассчитываем GrandTotalValueUSD для одного портфеля (он уже должен быть в portfolio.TotalValueUSD)
	// Этот шаг здесь избыточен, если portfolio.TotalValueUSD уже рассчитан сервисом, но для консистентности с APIPortfolioResponse...
	// Хотя для одного кошелька Data.GrandTotalValueUSD не очень осмысленно, если Portfolios - это один элемент.
	// Возможно, стоит вернуть просто *entity.WalletPortfolio без обертки APIPortfolioResponse для этого эндпоинта.
	// Пока оставим так для совместимости с ожидаемой структурой, где Data содержит Portfolios.
	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         []entity.WalletPortfolio{*portfolio}, // Оборачиваем в срез
			GrandTotalValueUSD: portfolio.TotalValueUSD,              // Grand total для одного это его же total
		},
		Errors: partialErrs, // Используем partialErrs, которые могут содержать ошибки по отдельным токенам/сетям
	}

	if len(partialErrs) > 0 {
		response.StatusMessage = "Portfolio retrieved. Some tokens or networks may have encountered errors."
	} else {
		response.StatusMessage = "Portfolio retrieved successfully."
	}

	c.JSON(http.StatusOK, response)
}

// GetSingleWalletNetworkPortfolioHandler handles requests for a specific network of a single wallet.
func (h *PortfolioHandler) GetSingleWalletNetworkPortfolioHandler(c *gin.Context) {
	ctx := c.Request.Context()
	walletAddress := c.Param("walletAddress")
	networkIdentifier := c.Param("networkIdentifier")

	h.logger.Debug("Handler GetSingleWalletNetworkPortfolioHandler called", "walletAddress", walletAddress, "networkIdentifier", networkIdentifier)

	// Для этого эндпоинта мы отслеживаем только одну указанную сеть
	trackedNetworkNames := []string{networkIdentifier}

	portfolio, partialErrs, err := h.portfolioService.FetchSingleWalletPortfolioByAddress(ctx, walletAddress, trackedNetworkNames)

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.logger.Warn("Data not found for single wallet/network portfolio request", "walletAddress", walletAddress, "network", networkIdentifier, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			h.logger.Error("Internal server error fetching single wallet/network portfolio", "walletAddress", walletAddress, "network", networkIdentifier, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: " + err.Error()})
		}
		return
	}

	// Проверяем, есть ли данные для запрошенной сети в портфолио
	// Это важно, так как FetchSingleWalletPortfolioByAddress может вернуть пустое BalancesByNetwork, если сеть отфильтровалась
	// или если для нее не было балансов. Ошибка "network not found" должна была быть поймана выше.
	// Однако, если сервис вернул портфолио, но без запрошенной сети (например, она неактивна), нужно это обработать.
	var finalPortfolioResponse *entity.WalletPortfolio = nil
	if portfolio != nil {
		// Если networkIdentifier был валидным и активным, то BalancesByNetwork в portfolio
		// должен содержать только эту сеть (или быть пустым, если балансов нет).
		// Мы ожидаем, что сервис уже отфильтровал сети.
		// Если BalancesByNetwork пустое, это может означать, что активных сетей не было (что покрывается err != nil выше)
		// или что для указанной сети нет балансов (что нормально).
		// Если же сервис вернул портфолио, но запрошенная сеть там отсутствует (а она должна быть единственной), это странно.
		// Однако, по текущей логике сервиса, если trackedNetworkNames содержит одну сеть, то activeNetworkDefinitions будет ее содержать (если она валидна),
		// и fetchSingleWalletPortfolio отработает по ней. Результат будет либо с данными, либо пустым для этой сети.

		// Проверим, действительно ли запрошенная сеть присутствует. Имя сети может отличаться от идентификатора.
		// Лучше полагаться, что сервис правильно отработал trackedNetworkNames.
		// Если portfolio.BalancesByNetwork содержит более одной сети или не содержит запрошенную, это проблема логики сервиса.
		// Для данного эндпоинта мы ожидаем, что *entity.WalletPortfolio будет содержать данные только по одной сети.
		finalPortfolioResponse = portfolio
		if _, networkDataExists := portfolio.BalancesByNetwork[networkIdentifier]; !networkDataExists {
			// Может быть, идентификатор сети в URL не совпадает с ключом в карте BalancesByNetwork (например, из-за регистра или Name vs Identifier)
			// Попробуем найти по идентификатору или имени в возвращенных данных (хотя должна быть только одна сеть)
			found := false
			for netName := range portfolio.BalancesByNetwork { // В норме здесь должна быть 0 или 1 сеть
				// Это не очень надежно, если netName не равен networkIdentifier. Сервис должен был это обеспечить.
				if strings.EqualFold(netName, networkIdentifier) {
					found = true
					break
				}
				// Дополнительная проверка: если в BalancesByNetwork есть ключ, соответствующий nd.Identifier из сервиса,
				// а в URL передали nd.Name. Это усложнение, лучше если сервис гарантирует, что ключ BalancesByNetwork
				// соответствует тому, что ожидается на основе trackedNetworkNames.
			}
			if !found && len(portfolio.BalancesByNetwork) > 0 {
				// Если есть какие-то данные по сетям, но не по той, что запросили, это странно, но вернем то, что есть,
				// или можно вернуть 404, если строго.
				// Для строгости можно вернуть 404, если конкретно запрошенная сеть отсутствует в результате.
				// c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("No balance data found for network %s in wallet %s", networkIdentifier, walletAddress)})
				// return
				h.logger.Warn("Portfolio data returned, but not for the specifically requested network", "requestedNetwork", networkIdentifier, "availableNetworksInPortfolio", portfolio.BalancesByNetwork)
			}
		}
	} else if err == nil { // portfolio is nil, but no critical error from service
		// Это может случиться, если fetchSingleWalletPortfolio возвращает (entity.WalletPortfolio{}, nil, nil), что не должно быть.
		// Он должен вернуть (*entity.WalletPortfolio, ..., nil)
		h.logger.Error("Service returned nil portfolio without critical error", "walletAddress", walletAddress, "network", networkIdentifier)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: inconsistent response from service"})
		return
	}

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         []entity.WalletPortfolio{*finalPortfolioResponse}, // Оборачиваем в срез
			GrandTotalValueUSD: finalPortfolioResponse.TotalValueUSD,              // Grand total для одного это его же total
		},
		Errors: partialErrs,
	}

	if len(partialErrs) > 0 {
		response.StatusMessage = "Portfolio for specific network retrieved. Some operations may have encountered errors."
	} else {
		response.StatusMessage = "Portfolio for specific network retrieved successfully."
	}

	c.JSON(http.StatusOK, response)
}
