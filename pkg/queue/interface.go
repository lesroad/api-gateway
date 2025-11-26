package queue

import (
	"api-gateway/model"
	"context"
)

type TaskQueue interface {
	Enqueue(task *model.Task) error
	Dequeue(ctx context.Context) (*model.Task, error)
	Size() int
	Close() error
}
