package mocktest

import (
	"testing"

	"github.com/cabify/runnerpool"
	"github.com/cabify/runnerpool/runnerpoolmock"
)

func TestMocksAreUpdated(t *testing.T) {
	// just try to compile this
	// this test is in a separate package to avoid testing `runnerpoolmock` itself, so it doesn't count for the coverage
	var _ runnerpool.Pool = &runnerpoolmock.Pool{}
	var _ runnerpool.Worker = &runnerpoolmock.Worker{}
}
