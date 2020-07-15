package runnerpool

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkCreatePool(b *testing.B) {
	for _, n := range []int{10, 50, 100, 1000, 5000} {
		b.Run(fmt.Sprintf("with %d workers", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cfg := Config{Workers: n}
				pool := New(cfg, goRunner)
				err := pool.Start()
				_ = err
				b.StopTimer()
				_ = pool.Stop(context.Background())
				b.StartTimer()
			}
		})
	}
}
