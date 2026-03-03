package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/errs"
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
			go w.pollOrderUntilFinal(context.Background(), orderID)
		}
	}
}

func (w *Worker) pollOrderUntilFinal(ctx context.Context, orderID string) {
	const (
		pollInterval = 5 * time.Second
		maxAttempts  = 20
	)
	attempt := 0
	for attempt < maxAttempts {
		select {
		case <-ctx.Done():
			return
		default:
			attempt++
			status, accrual, err := w.fetchAccrualFromExternal(orderID)
			if err != nil {
				time.Sleep(pollInterval)
				continue
			}
			isFinal := status == "PROCESSED" || status == "INVALID"
			if updateErr := w.storage.UpdateOrderStatus(ctx, orderID, status, accrual); updateErr != nil {
				w.logger.Errorw("failed to update order status", "orderID", orderID, "err", updateErr)
			}
			if isFinal {
				return
			}
			time.Sleep(pollInterval)
		}
	}
}

func (w *Worker) fetchAccrualFromExternal(orderID string) (string, float64, error) {
	url := fmt.Sprintf("%s/api/orders/%s", w.config.SystemAddress, orderID)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
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

	case http.StatusTooManyRequests:
		return "", 0, errs.ErrTooManyRequests

	default:
		return "", 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func (w *Worker) Stop() {
	close(w.processChan)
	w.wg.Wait()
}
