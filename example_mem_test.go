package ded_test

import (
	"fmt"
	"time"

	"github.com/mitranim/ded"
)

func ExampleMem_Dedup() {
	var mem ded.Mem

	first := mem.Dedup(ded.GetterFunc(slowork), ded.NowTimer{}, ded.Duration(time.Minute))
	second := mem.Dedup(ded.GetterFunc(slowork), ded.NowTimer{}, ded.Duration(time.Minute))

	// The value is reused without calling `slowork` again, keeping the old
	// timestamp, because it's not expired yet (a minute hasn't passed).
	fmt.Println(first == second)
	fmt.Println(first.Get())

	// Output:
	// true
	// some value
}

func slowork() interface{} { return `some value` }
