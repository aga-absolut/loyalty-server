package router

import (
	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/middleware/auth"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	"github.com/go-chi/chi"
)

func NewRouter(app *app.App) *chi.Mux {
	router := chi.NewRouter()
	router.Use(logger.WithLogging)
	router.Post("/api/user/register", app.RegisterHandler)
	router.Post("/api/user/login", app.AuthHandler)
	router.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.Post("/api/user/orders", app.AddOrderIDHandler)
		r.Get("/api/user/orders", app.GetListOrdersHandler)
		r.Get("/api/user/balance", app.GetBalanceHandler)
		r.Post("/api/user/balance/withdraw", app.WithdrawHandler)
		r.Get("/api/user/withdrawals", app.GetWithdraawalsHandler)
		r.Post("/api/user/accrual", app.AddAccrualHandler)
	})
	return router
}
