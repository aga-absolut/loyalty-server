package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/model"
	"github.com/aga-absolut/LoyaltyProgram/internal/repository"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

type Worker struct {
	processChan chan string
	storage     repository.Storage
	config      *config.Config
	logger      *logger.Logger
	wg          sync.WaitGroup
	size        int
}

func NewWorkerPool(ctx context.Context, processChan chan string, storage repository.Storage, size int, logger *logger.Logger, config *config.Config) *Worker {
	w := &Worker{
		processChan: processChan,
		storage:     storage,
		config:      config,
		size:        size,
		logger:      logger,
	}

	w.Start(ctx, size)
	return w
}

func (w *Worker) Start(ctx context.Context, size int) {
	w.wg.Add(size)
	for i := 0; i < size; i++ {
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
			status, accrual, err := w.fetchAccrualFromExternal(orderID)
			if err != nil {
				w.logger.Errorw("failed to fetch accrual", "orderID", orderID, "err", err)
				continue
			}
			err = w.storage.UpdateOrderStatus(ctx, orderID, status, accrual)
			if err != nil {
				w.logger.Errorw("failed to update order status", "orderID", orderID, "err", err)
				continue
			}
		}
	}
}

func (w *Worker) Stop() {
	close(w.processChan)
	w.wg.Wait()
}

func (w *Worker) fetchAccrualFromExternal(orderID string) (string, int, error) {
	url := fmt.Sprintf("%s/api/orders/%s", w.config.SystemAddress, orderID)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var accrualResp model.AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrualResp); err != nil {
			return "", 0, err
		}
		return accrualResp.Status, accrualResp.Accrual, nil
	case http.StatusNoContent:
		return "NOT_FOUND", 0, nil
	default:
		return "", 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}
