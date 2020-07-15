package runnerpool_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cabify/runnerpool"
)

func ExamplePool() {
	const workers = 2
	const tasks = workers + 1
	const timeout = 250 * time.Millisecond

	cfg := runnerpool.Config{
		Workers: workers,
	}

	runner := func(f func()) {
		go f()
	}

	pool := runnerpool.New(cfg, runner)
	err := pool.Start()
	if err != nil {
		log.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(tasks)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for i := 1; i <= tasks; i++ {
		go func(i int) {
			time.Sleep(time.Duration(i) * timeout / 10)

			worker, err := pool.Worker(ctx)
			if err != nil {
				fmt.Printf("Can't acquire worker %d\n", i)
				wg.Done()
				return
			}

			defer worker.Release()
			worker.Run(func(ctx context.Context) {
				time.Sleep(time.Duration(i) * timeout * 2)
				fmt.Printf("Worker %d done\n", i)
				wg.Done()
			})
		}(i)
	}

	wg.Wait()
	// Output:
	// Can't acquire worker 3
	// Worker 1 done
	// Worker 2 done
}
