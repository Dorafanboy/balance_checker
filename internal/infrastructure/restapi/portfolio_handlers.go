package restapi

import (
	"net/http"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/configloader"

	"github.com/gin-gonic/gin"
)

// APIPortfolioResponse определяет структуру ответа для эндпоинта портфелей.
type APIPortfolioResponse struct {
	Data struct {
		Portfolios []entity.WalletPortfolio `json:"portfolios"`
	} `json:"data"`
	ServiceErrors []entity.PortfolioError `json:"service_errors,omitempty"`
	StatusMessage string                  `json:"status_message"`
}

// PortfolioHandler обрабатывает HTTP запросы, связанные с портфелями.
type PortfolioHandler struct {
	portfolioService port.PortfolioService
	cfg              *configloader.Config
	// Можно добавить логгер, если потребуется специфичное логирование в хендлере
}

// NewPortfolioHandler создает новый экземпляр PortfolioHandler.
func NewPortfolioHandler(ps port.PortfolioService, cfg *configloader.Config) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioService: ps,
		cfg:              cfg,
	}
}

// GetPortfoliosHandler обрабатывает запрос на получение всех портфелей.
func (h *PortfolioHandler) GetPortfoliosHandler(c *gin.Context) {
	ctx := c.Request.Context() // Используем контекст из Gin запроса

	// Предполагаем, что cfg.TrackedNetworkIdentifiers уже корректно загружен и доступен
	// Если cfg nil или TrackedNetworkIdentifiers пуст, сервис сам должен это обработать или вернуть ошибку.
	// В нашем случае PortfolioService уже имеет логику для пустых trackedNetworkIdentifiers (вернет пустой результат и возможно лог).

	portfolios, serviceErrors := h.portfolioService.FetchAllWalletsPortfolio(ctx, h.cfg.TrackedNetworkIdentifiers)

	// Проверка на случай, если сам сервис вернул критическую ошибку (хотя FetchAllWalletsPortfolio спроектирован так,
	// чтобы возвращать ошибки в serviceErrors, а не через стандартный механизм ошибок Go).
	// Здесь можно добавить более сложную логику обработки ошибок, если FetchAllWalletsPortfolio может вернуть error.
	// Для текущей сигнатуры (возвращает portfolios, serviceErrors), мы всегда будем формировать ответ.

	response := APIPortfolioResponse{
		Data: struct {
			Portfolios []entity.WalletPortfolio `json:"portfolios"`
		}{Portfolios: portfolios},
		ServiceErrors: serviceErrors,
	}

	if len(serviceErrors) > 0 && len(portfolios) == 0 {
		response.StatusMessage = "Failed to retrieve any portfolios due to service errors."
		// Можно рассмотреть изменение HTTP статус кода, если нет ни одного портфеля и есть ошибки.
		// Например, http.StatusPartialContent или оставить http.StatusOK, но с явным сообщением.
		// Пока оставляем OK, но с сообщением.
	} else if len(serviceErrors) > 0 {
		response.StatusMessage = "Portfolios retrieved. Some wallets or tokens may have encountered errors."
	} else if len(portfolios) == 0 {
		// Это может случиться, если нет кошельков в wallets.txt или нет активных сетей/токенов.
		response.StatusMessage = "No portfolio data found. Check wallet list and network/token configurations."
	} else {
		response.StatusMessage = "Portfolios retrieved successfully."
	}

	c.JSON(http.StatusOK, response)
}
