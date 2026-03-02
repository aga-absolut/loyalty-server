package router

import (
	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/middleware/jwt"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	"github.com/go-chi/chi"
)

func NewRouter(app *app.App) *chi.Mux {
	router := chi.NewRouter()
	router.Use(logger.WithLogging)
	router.With().Post("/api/user/register", app.RegisterHandler)
	router.With().Post("/api/user/login", app.AuthHandler)
	router.With(jwt.AuthMiddleware).Post("/api/user/orders", app.AddOrderIDHandler)
	router.With(jwt.AuthMiddleware).Get("/api/user/orders", app.GetListOrdersHandler)
	router.With(jwt.AuthMiddleware).Get("/api/user/balance", app.GetBalanceHandler)
	router.With(jwt.AuthMiddleware).Post("/api/user/balance/withdraw", app.WithdrawHandler)
	router.With(jwt.AuthMiddleware).Get("/api/user/withdrawals", app.GetWithdraawalsHandler)
	router.With(jwt.AuthMiddleware).Post("/api/user/accrual", app.AddAccrualHandler)
	router.With(jwt.AuthMiddleware).Get("/api/orders/{number}", app.PollAccrualSystem)
	return router
}
