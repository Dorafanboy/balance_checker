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
type APIPortfolioData struct {
	WalletsPortfolio   []entity.WalletPortfolio `json:"wallets_portfolio"`
	GrandTotalValueUSD *float64                 `json:"grand_total_value_usd,omitempty"`
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
	ctx := c.Request.Context()

	var finalNetworkIdentifiers []string
	networksQueryParam := c.Query("networks")

	if networksQueryParam != "" {
		rawIdentifiers := strings.Split(networksQueryParam, ",")
		for _, id := range rawIdentifiers {
			trimmedID := strings.TrimSpace(id)
			if trimmedID != "" {
				finalNetworkIdentifiers = append(finalNetworkIdentifiers, trimmedID)
			}
		}
		h.logger.Debug("GetPortfoliosHandler: using 'networks' query parameter for filtering", "networks", finalNetworkIdentifiers)
	} else {
		if h.cfg != nil && len(h.cfg.Networks) > 0 {
			activeNetworkIdentifiers := make([]string, 0, len(h.cfg.Networks))
			for _, netCfg := range h.cfg.Networks {
				if netCfg.Name != "" {
					activeNetworkIdentifiers = append(activeNetworkIdentifiers, netCfg.Name)
				}
			}
			finalNetworkIdentifiers = activeNetworkIdentifiers
		}
		h.logger.Debug("GetPortfoliosHandler: 'networks' query parameter not provided, using all configured networks", "configured_networks_count", len(finalNetworkIdentifiers))
	}

	portfolios, serviceErrors := h.portfolioService.FetchAllWalletsPortfolio(ctx, finalNetworkIdentifiers)

	var grandTotal float64
	for _, p := range portfolios {
		grandTotal += p.TotalValueUSD
	}

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         portfolios,
			GrandTotalValueUSD: grandTotal,
		},
		Errors: serviceErrors,
	}

	if len(response.Errors) > 0 && len(portfolios) == 0 {
		response.StatusMessage = "Failed to retrieve any portfolios due to service errors."
	} else if len(response.Errors) > 0 {
		response.StatusMessage = "Portfolios retrieved. Some wallets or tokens may have encountered errors."
	} else if len(portfolios) == 0 {
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
	if failedWallets == nil {
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

	var trackedNetworkNames []string
	networksSingleQuery := c.Query("networks")
	if networksSingleQuery != "" {
		trackedNetworkNames = strings.Split(networksSingleQuery, ",")
		for i, name := range trackedNetworkNames {
			trackedNetworkNames[i] = strings.TrimSpace(name)
		}
	}

	h.logger.Debug("Handler GetSingleWalletPortfolioHandler called", "walletAddress", walletAddress, "trackedNetworksQuery", trackedNetworkNames)

	portfolio, partialErrs, err := h.portfolioService.FetchSingleWalletPortfolioByAddress(ctx, walletAddress, trackedNetworkNames)

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.logger.Warn("Data not found for single wallet portfolio request", "walletAddress", walletAddress, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			h.logger.Error("Internal server error fetching single wallet portfolio", "walletAddress", walletAddress, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: " + err.Error()})
		}
		return
	}

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         []entity.WalletPortfolio{*portfolio},
			GrandTotalValueUSD: portfolio.TotalValueUSD,
		},
		Errors: partialErrs,
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

	var finalPortfolioResponse *entity.WalletPortfolio = nil
	if portfolio != nil {
		finalPortfolioResponse = portfolio
		if _, networkDataExists := portfolio.BalancesByNetwork[networkIdentifier]; !networkDataExists {
			found := false
			for netName := range portfolio.BalancesByNetwork {
				if strings.EqualFold(netName, networkIdentifier) {
					found = true
					break
				}
			}
			if !found && len(portfolio.BalancesByNetwork) > 0 {
				h.logger.Warn("Portfolio data returned, but not for the specifically requested network", "requestedNetwork", networkIdentifier, "availableNetworksInPortfolio", portfolio.BalancesByNetwork)
			}
		}
	} else if err == nil {
		h.logger.Error("Service returned nil portfolio without critical error", "walletAddress", walletAddress, "network", networkIdentifier)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: inconsistent response from service"})
		return
	}

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios         []entity.WalletPortfolio `json:"portfolios"`
			GrandTotalValueUSD float64                  `json:"grandTotalValueUSD"`
		}{
			Portfolios:         []entity.WalletPortfolio{*finalPortfolioResponse},
			GrandTotalValueUSD: finalPortfolioResponse.TotalValueUSD,
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
