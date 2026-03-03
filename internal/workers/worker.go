package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func NewPollWorker(ctx context.Context, processChan chan string, storage repository.Storage, size int, logger *logger.Logger, config *config.Config) *Worker {
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
	var (
		tout = time.After(2 * time.Minute)
		pollInterval = 1 * time.Second
		maxAttempts  = 12
		attempt = 0
	)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt++

			if attempt > maxAttempts {
				w.logger.Errorw("No more than 12 requests per minute allowed")
			}

			status, accrual, err := w.fetchAccrualFromExternal(ctx, orderID)
			if err != nil {
				w.logger.Errorw("failed to fetch accrual", "orderID", orderID, "err", err)
			}
			if status == "PROCESSED" {
				if updateErr := w.storage.UpdateOrderStatus(ctx, orderID, status, accrual); updateErr != nil {
					w.logger.Errorw("failed to update order status",
						"orderID", orderID,
						"err", updateErr,
					)
				}
			} else {
				fmt.Println(status, " not equal PROCESSED!")
			}

		case <-tout:
			w.logger.Info("timeout")
			return
		}
	}
}

func (w *Worker) fetchAccrualFromExternal(ctx context.Context, orderID string) (string, float64, error) {
	var response model.AccrualResponse
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read response error:", err)
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("read response error:", err)
	}
	return response.Status, response.Accrual, nil
}

func (w *Worker) Stop() {
	close(w.processChan)
	w.wg.Wait()
}
