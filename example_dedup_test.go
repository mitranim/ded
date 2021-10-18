package ded_test

import (
	"fmt"

	"github.com/mitranim/ded"
)

func ExampleDedup() {
	var worker Worker
	first := worker.Fetch()
	second := worker.Fetch()

	// The value is reused without calling `(*Worker).Get` again, keeping the old
	// timestamp, because it's not expired yet (a minute hasn't passed).
	fmt.Println(first == second)
	fmt.Println(first.Get())

	// Output:
	// true
	// some value
}

// The fields `ExpireMinute` and `NowTimer` are zero-sized and only provide us
// with methods required for `Dedup`. Only `Mem` has an actual memory cost.
type Worker struct {
	ded.ExpireMinute
	ded.NowTimer
	ded.Mem
}

// May reuse the cached value.
func (self *Worker) Fetch() ded.Timed { return ded.Dedup(self) }

// Could be slow, expensive work. Not always called.
func (self *Worker) Get() interface{} { return `some value` }
