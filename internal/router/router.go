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
	router.Route("/api/user", func(r chi.Router) {

		r.With(compress.Decompress).Post("/register", app.RegisterHandler)
		r.With(compress.Decompress).Post("/login", app.AuthHandler)

		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware)

			r.With(compress.Decompress).Post("/orders", app.AddOrderIDHandler)
			r.With(compress.Compress).Get("/orders", app.GetListOrdersHandler)
			r.With(compress.Compress).Get("/balance", app.GetBalanceHandler)
			r.With(compress.Decompress).Post("/balance/withdraw", app.WithdrawHandler)
			r.With(compress.Compress).Get("/withdrawals", app.GetWithdrawalsHandler)
		})
	})
	return router
}
