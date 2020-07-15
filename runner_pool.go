package runnerpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Pool represents a pool of runners, understanding runners as func(func()) functions like func(f func()) { go f() }
type Pool interface {
	// Worker returns a Worker if can be acquired before context is canceled, otherwise returns the
	// ctx.Err() result
	Worker(ctx context.Context) (Worker, error)

	// Stats provides statistics about pool's size, status & config
	Stats() Stats
}

// Stats provides statistics about pool's size, status & config
type Stats struct {
	MaxWorkers int32
	Workers    int32
	Acquired   int32
	Running    int32
}

//go:generate mockery -outpkg runnerpoolmock -output runnerpoolmock -case underscore -name Pool
var _ Pool = &WorkerPool{}

// Worker represents a pool worker that offers a one-time usable runner
type Worker interface {
	// Run runs the given function.
	// The context provided is will be canceled if WorkerPool.Stop is called.
	Run(func(context.Context))

	// Release should be called at least once every time a worker is acquired
	// It is safe to call Release twice and calling Release on a worker that is already running has no effect,
	// so the safest way is to defer worker.Release() once the worker has been acquired.
	Release()
}

//go:generate mockery -outpkg runnerpoolmock -output runnerpoolmock -case underscore -name Worker
var _ Worker = &worker{}

// New returns a new WorkerPool, this should be started after being used
// Run should be provided to choose whether to recover or report panics, notice that if you
// recover from panics, the workers won't be restablished so you'll eventually exhaust them
func New(cfg Config, runner func(func())) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		config: cfg,
		runner: runner,
		sem:    make(chan struct{}, cfg.Workers),

		ctx:  ctx,
		stop: cancel,
	}
}

// Config is the configuration for the pool implementation
type Config struct {
	Workers int `default:"100"`
}

// WorkerPool implements Pool using a runner & goroutines pool
type WorkerPool struct {
	workers chan *worker

	config Config

	runner func(func())

	stats struct {
		workers  int32
		acquired int32
		running  int32
	}

	sem chan struct{}

	ctx  context.Context
	stop func()
}

type worker struct {
	f        chan func(context.Context)
	once     sync.Once
	released chan struct{}
}

func (w *worker) Run(f func(context.Context)) {
	w.f <- f
}

func (w *worker) Release() {
	w.once.Do(func() { close(w.released) })
}

// Worker returns an error, if possible within the provided context, otherwise it will return ctx.Err()
func (p *WorkerPool) Worker(ctx context.Context) (Worker, error) {
	select {
	case w := <-p.workers:
		return w, nil

	case <-ctx.Done():
		return nil, ErrCantAcquireWorker{ctx.Err()}

	case <-p.ctx.Done():
		return nil, ErrCantAcquireWorker{fmt.Errorf("pool is already stopped")}
	}
}

// Start starts creating the workers
func (p *WorkerPool) Start() error {
	if p.config.Workers == 0 {
		return fmt.Errorf("can't start a pool with 0 workers")
	}

	p.workers = make(chan *worker)

	go func() {
		for {
			// We force the loop to always evaluate first if the pool is
			// being stopped before trying to create a worker
			select {
			case <-p.ctx.Done():
				return
			default:
				select {
				case p.sem <- struct{}{}:
					p.createWorker()
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// Stats provides thread-safe statistics about pool's size, status & config
func (p *WorkerPool) Stats() Stats {
	return Stats{
		MaxWorkers: int32(p.config.Workers),
		Workers:    atomic.LoadInt32(&p.stats.workers),
		Acquired:   atomic.LoadInt32(&p.stats.acquired),
		Running:    atomic.LoadInt32(&p.stats.running),
	}
}

func (p *WorkerPool) createWorker() {
	p.inc(&p.stats.workers)
	p.runner(func() {
		defer func() { <-p.sem }()
		defer p.dec(&p.stats.workers)

		for {
			worker := &worker{
				f:        make(chan func(context.Context)),
				released: make(chan struct{}),
			}

			select {
			case p.workers <- worker:
				func() {
					p.inc(&p.stats.acquired)
					defer p.dec(&p.stats.acquired)

					select {
					case f := <-worker.f:
						func() {
							p.inc(&p.stats.running)
							defer p.dec(&p.stats.running)
							f(p.ctx)
						}()
					case <-worker.released:
						// keep looping
					}
				}()

			case <-p.ctx.Done():
				return
			}
		}
	})
}

func (p *WorkerPool) inc(ptr *int32) {
	atomic.AddInt32(ptr, 1)
}

func (p *WorkerPool) dec(ptr *int32) {
	atomic.AddInt32(ptr, -1)
}

// Stop cancel the pool's context which is provided to each worker and will wait until all workers are stopped or until
// context is done, in which case it will return a wrapped ctx.Err()
func (p *WorkerPool) Stop(ctx context.Context) error {
	p.stop()

	// It tries to push as many times as workers are defined for the pool.
	// If it succeeds, it means all elements inside sem chan belong to the Stop
	// call and, thus, all workers are stopped.
	for i := 0; i < p.config.Workers; i++ {
		select {
		case p.sem <- struct{}{}:
		// we're good
		case <-ctx.Done():
			return fmt.Errorf("can't stop all workers: %w", ctx.Err())
		}
	}
	return nil
}

// ErrCantAcquireWorker is returned by Pool.Worker() function when worker can't be acquired
type ErrCantAcquireWorker struct {
	cause error
}

// Unwrap implements the errors unwrapping
func (e ErrCantAcquireWorker) Unwrap() error {
	return e.cause
}

func (e ErrCantAcquireWorker) Error() string {
	return fmt.Sprintf("can't acquire worker: %s", e.cause)
}
