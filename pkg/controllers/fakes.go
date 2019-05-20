package controllers

import "context"

type fakeGenericController struct {
	runError     error
	enqueueError error
	queue        []interface{}
	tombstone    []interface{}
}

func newFakeGenericController() *fakeGenericController {
	return &fakeGenericController{
		queue:     make([]interface{}, 0),
		tombstone: make([]interface{}, 0),
	}
}

func (c *fakeGenericController) Run(ctx context.Context) error {
	return c.runError
}

func (c *fakeGenericController) enqueue(obj interface{}) {
	c.queue = append(c.queue, obj)
}

func (c *fakeGenericController) enqueueTombstone(obj interface{}) {
	c.tombstone = append(c.tombstone, obj)
}
