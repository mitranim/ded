## Overview

`ded`: short for "dedup". Experimental tool for deduplicating concurrent background operations in Go, with support for limited-time caching.

API docs: https://pkg.go.dev/github.com/mitranim/ded.

Main primitive is `Mem` (see docs). Features:

  * Stores arbitrary value and its timestamp.
  * Provides concurrent read access, using `sync.RWMutex`.
  * Readers can independently decide if the value is expired.
  * When the value is expired, a reader gets upgraded to a writer, producing a new value.
  * Readers don't wait for each other.
  * Readers wait for the writer, if any.
  * There is little overhead.

The current design uses blocking via `sync.RWMutex`, without support for channels or context. The main reason is efficiency. To support channels, each newly-cached value would have to be wrapped in a new "future" with a new channel, and work would have to be done on a new background goroutine. That's a lot of overhead for a single value. The current design is much more efficient, with no mandatory allocations per value.

## Usage

The recommended way is type-oriented, by embedding relevant types in your own. All `ded` types are usable when zero-initialized, and don't require constructors.

Many `ded` types are zero-sized, and can be embedded in other types to provide additional methods at no memory cost, _when fields are in the right order_. Beware of the padding gotcha: https://dave.cheney.net/2015/10/09/padding-is-hard. Zero-sized fields should come first.

```golang
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
```

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
