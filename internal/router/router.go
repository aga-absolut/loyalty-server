package router

import (
	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/middleware/auth"
	"github.com/aga-absolut/LoyaltyProgram/middleware/compress"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	"github.com/go-chi/chi"
)

func NewRouter(app *app.App) *chi.Mux {
	router := chi.NewRouter()
	router.Use(logger.WithLogging)
	router.With(compress.Decompress).Post("/api/user/register", app.RegisterHandler)
	router.With(compress.Decompress).Post("/api/user/login", app.AuthHandler)
	router.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.With(compress.Decompress).Post("/api/user/orders", app.AddOrderIDHandler)
		r.With(compress.Compress).Get("/api/user/orders", app.GetListOrdersHandler)
		r.With(compress.Compress).Get("/api/user/balance", app.GetBalanceHandler)
		r.With(compress.Decompress).Post("/api/user/balance/withdraw", app.WithdrawHandler)
		r.With(compress.Compress).Get("/api/user/withdrawals", app.GetWithdrawalsHandler)
	})
	return router
}
