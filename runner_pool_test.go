package runnerpool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
)

func TestWorkerPool(t *testing.T) {
	t.Run("performing parallel tasks", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 2}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)
		defer pool.Stop(context.Background())

		ch1, ch2 := make(chan struct{}), make(chan struct{})

		func() {
			worker, err := pool.Worker(context.Background())
			assert.NoError(t, err)
			defer worker.Release()

			worker.Run(func(_ context.Context) {
				close(ch1)
				<-ch2
			})
		}()

		func() {
			worker, err := pool.Worker(context.Background())
			assert.NoError(t, err)
			defer worker.Release()

			worker.Run(func(_ context.Context) {
				close(ch2)
				<-ch1
			})
		}()

		select {
		case <-ch1:
		case <-time.After(time.Second):
			t.Errorf("ch1 should be closed")
		}

		select {
		case <-ch2:
		case <-time.After(time.Second):
			t.Errorf("ch2 should be closed")
		}
	})

	t.Run("worker's context is canceled when stopping", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 1}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)

		worker, err := pool.Worker(context.Background())
		assert.NoError(t, err)
		defer worker.Release()

		workerStopped := make(chan struct{})

		worker.Run(func(ctx context.Context) {
			<-ctx.Done()
			close(workerStopped)
		})

		err = pool.Stop(context.Background())
		assert.NoError(t, err)

		select {
		case <-workerStopped:
		case <-time.After(time.Second):
			t.Errorf("Worker didn't stop when pool.Stop was called")
		}
	})

	t.Run("stop returns a wrapped ctx.Err() when worker won't stop", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 1}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)

		worker, err := pool.Worker(context.Background())
		assert.NoError(t, err)
		defer worker.Release()

		block := make(chan struct{})
		defer close(block) // don't leak this after test has ended
		worker.Run(func(_ context.Context) {
			<-block
		})

		// this can be flaky letting broken code pass, but can't give false positives
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = pool.Stop(ctx)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Error should wrap context.DeadlineExceeded, but it is: %s", err)
	})

	t.Run("rejecting when no workers are available", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 1}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)
		defer pool.Stop(context.Background())

		unblock, unblocked := runBlockingWorker(t, pool)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = pool.Worker(ctx)
		assert.NotNil(t, err)
		assert.IsType(t, ErrCantAcquireWorker{}, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "Error doesn't wrap cause correctly")

		close(unblock)
		<-unblocked
	})

	t.Run("releasing a worker without running anything", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 1}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)

		func() {
			worker, err := pool.Worker(context.Background())
			assert.NoError(t, err)
			worker.Release()
		}()

		// just try to stop, if can't or there're leaked goroutines, then we failed releasing
		pool.Stop(context.Background())
	})

	t.Run("stats", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 10}

		pool := New(cfg, goRunner)
		err := pool.Start()
		assert.NoError(t, err)
		defer pool.Stop(context.Background())

		unblockFirst, unblockedFirst := runBlockingWorker(t, pool)
		unblockSecond, unblockedSecond := runBlockingWorker(t, pool)

		acquiredWorker, err := pool.Worker(context.Background())
		assert.NoError(t, err)
		defer acquiredWorker.Release()

		close(unblockFirst)
		<-unblockedFirst

		// let the numbers update, we can't know when counter is decreased after first worker has been freed
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, Stats{
			MaxWorkers: int32(cfg.Workers),
			Workers:    10, // all the workers are created
			Acquired:   2,  // second is running, and another one is just acquired
			Running:    1,  // only second is running now
		}, pool.Stats())

		close(unblockSecond)
		<-unblockedSecond
	})

	t.Run("stats are okay even if we panic", func(t *testing.T) {
		defer leaktest.Check(t)()

		cfg := Config{Workers: 10}

		pool := New(cfg, recoverFromPanics)

		err := pool.Start()
		assert.NoError(t, err)
		defer pool.Stop(context.Background())

		worker, err := pool.Worker(context.Background())
		assert.NoError(t, err)
		worker.Run(func(_ context.Context) { panic("roto, todo roto") })
		worker.Release()

		// let the numbers update, we can't know when counter is decreased after first worker has been freed
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, Stats{
			MaxWorkers: int32(cfg.Workers),
			Workers:    int32(cfg.Workers), // panicked worker has been created again
			Acquired:   0,
			Running:    0,
		}, pool.Stats())
	})
}

func runBlockingWorker(t *testing.T, pool Pool) (unblock, unblocked chan struct{}) {
	unblock = make(chan struct{})
	unblocked = make(chan struct{})

	func() {
		worker, err := pool.Worker(context.Background())
		assert.NoError(t, err)
		defer worker.Release()

		worker.Run(func(_ context.Context) {
			<-unblock
			close(unblocked)
		})
	}()

	return unblock, unblocked
}

func goRunner(f func()) { go f() }

func recoverFromPanics(f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// recover the panic
			}
		}()
		f()
	}()
}
