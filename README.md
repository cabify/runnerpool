# runnerpool

Library that offers a bounded pool of goroutines to perform deserved jobs.

# Usage
First of all, you need to create a pool with some configuration and a _runner_ 
```go
    cfg := runnerpool.Config{
        Workers: workers,
    }
    
    pool := runnerpool.New(cfg, runner)
```

A _runner_ is a function that starts goroutines. Why do we need one? Because you probably want to handle panics in those goroutines in your favourite way.

Could you just `defer` a recovery from panics in every function you pass to the workers? Yes, that would be another solution, but we chose this one.

We'll use a runner that just launches goroutines:

```go
    runner := func(f func()) {
        go f()
    }
```

Now we need to start the runnerpool, so it will create the goroutines. Notice that it will create them all from the beginning.
This process doesn't take _much time_, it takes around 3ms to create 5000 goroutines, which should not worry you in your app startup.
Anyway, you can check `runner_pool_bench_test.go` for more benchmarks.

Once you have the pool started, you can acquire a worker. You'll need a provide a context for that, because usually you'll want to desist from waiting at some point.

```go
    worker, err := pool.Worker(ctx)
    if err != nil {
        return err
    }
    defer worker.Release()
```

If error is returned then it may wrap one of the errors returned by `ctx.Err()` (or not, if the pool was just stopped).

Once you've successfully acquired a worker, you should make sure you'll return it back to the pool when you leave, 
regardless you're going to execute code on it or not, just `defer worker.Release()`. This is a no-op once you've run 
some code.

Now, you can run code on the worker:
```go
    worker.Run(func(_ context.Context) { fmt.Println("I'm running!") })
```

Notice that a worker can't execute code twice, and it obviously can't execute code once it has been released.

The context provided to the worker will be canceled if pool's `Stop(ctx)` method is called, and the `Stop(ctx)` will wait 
until all workers are stopped or the provided context is done.
Once pool's `Stop()` method is called, no more workers can be acquired and `Worker(ctx)` will return an error. 
