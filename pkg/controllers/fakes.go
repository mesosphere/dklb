package controllers

import (
	"context"
	"sync"
)

type fakeGenericController struct {
	mutex        *sync.Mutex
	runError     error
	enqueueError error
	queue        []interface{}
	tombstone    []interface{}
}

func newFakeGenericController() *fakeGenericController {
	return &fakeGenericController{
		mutex:     &sync.Mutex{},
		queue:     make([]interface{}, 0),
		tombstone: make([]interface{}, 0),
	}
}

func (c *fakeGenericController) Run(ctx context.Context) error {
	return c.runError
}

func (c *fakeGenericController) enqueue(obj interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.queue = append(c.queue, obj)
}

func (c *fakeGenericController) enqueueTombstone(obj interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.tombstone = append(c.tombstone, obj)
}
