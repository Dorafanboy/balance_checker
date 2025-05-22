package restapi

import (
	"github.com/gin-gonic/gin"
	_ "github.com/json-iterator/go"
)

// SetupRouter настраивает и возвращает экземпляр Gin роутера.
func SetupRouter(portfolioAPIHandler *PortfolioHandler) *gin.Engine {
	router := gin.Default()

	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/portfolios", portfolioAPIHandler.GetPortfoliosHandler)
		apiV1.GET("/portfolios/failed", portfolioAPIHandler.GetFailedWalletsHandler)

		apiV1.GET("/portfolios/:walletAddress", portfolioAPIHandler.GetSingleWalletPortfolioHandler)

		apiV1.GET("/portfolios/:walletAddress/networks/:networkIdentifier", portfolioAPIHandler.GetSingleWalletNetworkPortfolioHandler)
	}

	return router
}
