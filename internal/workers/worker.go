package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/models"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

type Worker struct {
	processChan chan string
	storage     app.Storage
	config      *config.Config
	logger      *logger.Logger
	wg          sync.WaitGroup
	size        int
}

func NewPollWorker(ctx context.Context, processChan chan string, storage app.Storage, size int, logger *logger.Logger, config *config.Config) *Worker {
	w := &Worker{
		processChan: processChan,
		storage:     storage,
		config:      config,
		size:        size,
		logger:      logger,
	}

	w.Start(ctx)
	return w
}

func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(w.size)
	for i := 0; i < w.size; i++ {
		go w.worker(ctx)
	}
}

func (w *Worker) worker(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case orderID, ok := <-w.processChan:
			if !ok {
				return
			}

			w.pollOrderUntilFinal(ctx, orderID)
		}
	}
}

func (w *Worker) pollOrderUntilFinal(ctx context.Context, orderID string) {
	tout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, accrual, err := w.fetchAccrualFromExternal(ctx, orderID)
			if err != nil {
				w.logger.Errorw("failed to fetch accrual", "orderID", orderID, "err", err)
				continue
			}
			if updateErr := w.storage.UpdateOrderStatus(ctx, orderID, status, accrual); updateErr != nil {
				w.logger.Errorw("failed to update order status",
					"orderID", orderID,
					"err", updateErr,
				)
			}

			if status == "PROCESSED" || status == "INVALID" {
				w.logger.Infow("Order processing finished",
					"orderID", orderID,
					"status", status,
					"accrual", accrual,
				)
				return
			}

		case <-tout:
			w.logger.Infow("Polling timeout reached", "orderID", orderID)
			return
		}
	}
}

func (w *Worker) fetchAccrualFromExternal(ctx context.Context, orderID string) (string, float64, error) {
	var response models.AccrualResponse
	url := fmt.Sprintf("%s/api/orders/%s", w.config.SystemAddress, orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", 0, fmt.Errorf("failed to decode accrual response: %w", err)
	}
	return response.Status, response.Accrual, nil
}

func (w *Worker) Stop() {
	close(w.processChan)
	w.wg.Wait()
}
