package restapi

import (
	"github.com/gin-gonic/gin"
)

// SetupRouter настраивает и возвращает экземпляр Gin роутера.
func SetupRouter(portfolioHandler *PortfolioHandler) *gin.Engine {
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
