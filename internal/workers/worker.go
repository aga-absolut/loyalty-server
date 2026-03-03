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
			go w.Poll(context.Background(), order)
		}
	}
}

func (w *Worker) Poll(ctx context.Context, order model.TypeForChannel) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	timeout := time.After(2 * time.Minute)
	attempts := 0
	maxAttempts := 12
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var response model.AccrualResponse
			attempts++

			if attempts > maxAttempts {
				return fmt.Errorf("превышено количество попыток")
			}
			url := fmt.Sprintf("%s/api/orders/%s", w.accrual, order.OrderNum)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				fmt.Println("req create error:", err)
				continue
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("poll error:", err)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Println("read response error:", err)
				continue
			}

			err = json.Unmarshal(body, &response)
			if err != nil {
				return fmt.Errorf("ошибка парсинга: ")
			}

			if response.Status == "PROCESSED" {
				err = w.storage.UpdateOrderProgress(ctx, response.Status, response.Order, order.User, float64(response.Accrual), 0)
				if err != nil {
					return fmt.Errorf("error in update db: ")
				}
				return nil
			} else {
				fmt.Println(response.Status, " not equal PROCESSED!")
			}
		case <-timeout:
			return fmt.Errorf("time is out")
		}
	}
}

func (w *Worker) StopWorker() {
	w.wg.Wait()
}
