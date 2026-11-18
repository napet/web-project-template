package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func healthCheck(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"message": "Server OK"})
}
