package restapi

import (
	"github.com/gin-gonic/gin"
	_ "github.com/json-iterator/go" // ADDED: Import for potential global effects or direct use later
)

// SetupRouter настраивает и возвращает экземпляр Gin роутера.
func SetupRouter(portfolioHandler *PortfolioHandler) *gin.Engine {
	// gin.EnableJsonDecoderUseNumber() // Consider enabling if numbers need to be decoded as json.Number.
	// For now, let's not enable it unless specifically needed, as it changes how numbers are decoded.

	router := gin.Default() // Используем gin.Default() для включения стандартных middleware (Logger, Recovery)

	// Группа для API v1
	v1 := router.Group("/api/v1")
	{
		v1.GET("/portfolios", portfolioHandler.GetPortfoliosHandler)
		// Здесь можно будет добавлять другие ручки для v1
	}

	// Можно добавить и другие группы или ручки верхнего уровня, если потребуется
	// router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	return router
}
