package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/aga-absolut/LoyaltyProgram/internal/model"
	"github.com/aga-absolut/LoyaltyProgram/internal/repository"
)

type Worker struct {
	pollCh  chan model.TypeForChannel
	storage repository.Storage
	accrual string
	wg      sync.WaitGroup
}

func NewPollWorker(ctx context.Context, accrual string, storage repository.Storage, pollCh chan model.TypeForChannel) *Worker {
	worker := &Worker{
		storage: storage,
		pollCh:  pollCh,
		accrual: accrual,
	}

	worker.wg.Add(1)
	go worker.PollOrderStatus(ctx)

	return worker
}
func (w *Worker) PollOrderStatus(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-w.pollCh:
			if !ok {
				return
			}
			// ← Вот главное: асинхронный запуск polling'а
			go w.pollAsync(context.Background(), order)
		}
	}
}

// Новый метод: асинхронный polling одного заказа
func (w *Worker) pollAsync(ctx context.Context, order model.TypeForChannel) {
	const pollInterval = 5 * time.Second // для тестов достаточно 5 сек
	const maxAttempts = 20              // ~100 сек

	attempts := 0

	for attempts < maxAttempts {
		attempts++

		// Проверяем отмену перед запросом
		select {
		case <-ctx.Done():
			return
		default:
		}

		url := fmt.Sprintf("%s/api/orders/%s", w.accrual, order.OrderNum)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			time.Sleep(pollInterval) // временно — лучше заменить ниже
			continue
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var response model.AccrualResponse
		if err := json.Unmarshal(body, &response); err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if response.Status == "PROCESSED" {
			err = w.storage.UpdateOrderProgress(ctx, response.Status, response.Order, order.User, float64(response.Accrual), 0)
			if err != nil {
				// логгируй ошибку, но не останавливай
			}
			return // успех — выходим
		}

		// Не PROCESSED — ждём, но отменяемо
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			// продолжаем
		}
	}
	// max attempts — можно пометить заказ как FAILED
}

func (w *Worker) StopWorker() {
	w.wg.Wait()
}
