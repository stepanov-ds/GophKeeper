package router

import (
	"github.com/gin-gonic/gin"
	"github.com/stepanov-ds/GophKeeper/internal/config"
	"github.com/stepanov-ds/GophKeeper/internal/handlers"
	"github.com/stepanov-ds/GophKeeper/internal/handlers/middlewares"
	"github.com/stepanov-ds/GophKeeper/internal/utils"
)

// Устанавливает маршруты
func Route(r *gin.Engine) {
	r.RedirectTrailingSlash = true
	if *config.RegistrationEnabled {
		r.POST("/register", func(ctx *gin.Context) {
			handlers.Register(ctx)
		})
	}
	cache := utils.NewMemoryCache(*config.CleanupTime)

	r.GET("/login", func(ctx *gin.Context) {
		handlers.LoginGet(ctx, cache)
	})
	r.POST("/login", func(ctx *gin.Context) {
		handlers.LoginPost(ctx, cache)
	})


	r.POST("/update", middlewares.AuthMiddleware(), func(ctx *gin.Context) {
		handlers.Update(ctx)
	})
	r.POST("/sync", middlewares.AuthMiddleware(), func(ctx *gin.Context) {
		handlers.Sync(ctx)
	})
}
