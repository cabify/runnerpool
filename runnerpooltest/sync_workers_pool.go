/*
Package runnerpooltest provides an implementation of runnerpool.Pool to be used from tests of third party libraries
where we just need to provide a dependency that will execute the code synchronously.
For fine-grained control of the Pool, use the package runnerpoolmock
*/
package runnerpooltest

import (
	"context"

	"github.com/cabify/runnerpool"
)

// SyncWorkerPool provides workers that run the provided function synchronously.
// This is used to test third party libraries depending on runnerpool.Pool
type SyncWorkersPool struct{}

// Worker always provides a worker with no calls limit
func (s SyncWorkersPool) Worker(ctx context.Context) (runnerpool.Worker, error) {
	return syncWorker{}, nil
}

// Stats does not provide correct stats for this test implementation
func (s SyncWorkersPool) Stats() runnerpool.Stats { return runnerpool.Stats{} }

type syncWorker struct{}

func (s syncWorker) Run(f func(context.Context)) { f(context.Background()) }
func (s syncWorker) Release()                    {}
