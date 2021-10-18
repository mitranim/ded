package ded

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func Test_Either_Get(t *testing.T) {
	for _, val := range testVals {
		testGet(t, val, Either{val})
	}
}

func Test_Either_Unwrap(t *testing.T) {
	test := func(val interface{}, expVal interface{}, expErr error) {
		outVal, outErr := Either{val}.Unwrap()

		eq(t, expVal, outVal)
		eq(t, expErr, outErr)
	}

	test(nil, nil, nil)
	test(10, 10, nil)
	test(`val`, `val`, nil)

	err := testErr()
	test(err, nil, err)
}

func Test_Either_Set(t *testing.T) {
	test := func(val interface{}) {
		var tar Either
		tar.Set(val)
		eq(t, Either{val}, tar)
	}

	for _, val := range testVals {
		test(val)
	}
}

func Test_Either_SetGetter(t *testing.T) {
	test := func(val interface{}) {
		var tar Either
		tar.SetGetter(Either{val})
		eq(t, Either{val}, tar)
	}

	for _, val := range testVals {
		test(val)
	}
}

func Test_Timed_SetTimer_nil(t *testing.T) {
	for _, val := range testVals {
		for _, inst := range testTimes {
			tar := Timed{Either{val}, inst}
			tar.SetTimer(nil)
			eq(t, Timed{Either{val}, time.Time{}}, tar)
		}
	}
}

func Test_Timed_SetTimer_non_nil(t *testing.T) {
	for _, val := range testVals {
		inst0 := time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC)
		inst1 := time.Date(2, 3, 4, 5, 6, 7, 8, time.UTC)

		tar := Timed{Either{val}, inst0}
		tar.SetTimer(Inst(inst1))

		eq(t, Timed{Either{val}, inst1}, tar)
	}
}

func Test_NewMem(t *testing.T) {
	for _, val := range testVals {
		for _, inst := range testTimes {
			eq(t, &Mem{val: Timed{Either{val}, inst}}, NewMem(MakeTimed(val, inst)))
		}
	}
}

func Test_Mem_Get(t *testing.T) {
	for _, val := range testVals {
		testGet(t, val, NewMem(MakeTimed(val, time.Time{})))
	}
}

func Test_Mem_GetTimed(t *testing.T) {
	for _, val := range testVals {
		for _, inst := range testTimes {
			eq(t, Timed{Either{val}, inst}, NewMem(MakeTimed(val, inst)).GetTimed())
		}
	}
}

func Test_Mem_SetTimed(t *testing.T) {
	for _, val := range testVals {
		for _, inst := range testTimes {
			var mem Mem
			mem.SetTimed(Timed{Either{val}, inst})
			eq(t, Timed{Either{val}, inst}, mem.GetTimed())
		}
	}
}

func Test_Mem_Zero(t *testing.T) {
	for _, val := range testVals {
		for _, inst := range testTimes {
			mem := NewMem(MakeTimed(val, inst))
			mem.Zero()
			eq(t, Timed{}, mem.GetTimed())
		}
	}
}

func Test_Mem_Dedup_from_zero(t *testing.T) {
	for _, getter := range testGetters {
		for _, timer := range testTimers {
			for _, expirer := range testExpirers {
				var mem Mem
				var expected Timed

				if IsExpired(expirer, mem.GetTimed()) {
					expected.SetGetter(getter)
					expected.SetTimer(timer)
				}

				actual := mem.Dedup(getter, timer, expirer)

				eq(t, actual, mem.GetTimed())

				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf(`
getter:
	%#[1]v
timer:
	%#[2]v
expirer:
	%#[3]v
expected %[4]T (detailed):
	%#[4]v
actual %[5]T (detailed):
	%#[5]v
expected %[4]T (simple):
	%[4]v
actual %[5]T (simple):
	%[5]v
`, getter, timer, expirer, expected, actual)
				}
			}
		}
	}
}

func Test_Mem_Dedup_from_non_zero(t *testing.T) {
	for _, getter0 := range testGetters {
		for _, timer0 := range testTimers {
			for _, expirer0 := range testExpirers {
				for _, getter1 := range testGetters {
					for _, timer1 := range testTimers {
						for _, expirer1 := range testExpirers {
							var mem Mem
							mem.Dedup(getter0, timer0, expirer0)

							var expected Timed

							if IsExpired(expirer1, mem.GetTimed()) {
								expected.SetGetter(getter1)
								expected.SetTimer(timer1)
							} else {
								expected = mem.GetTimed()
							}

							actual := mem.Dedup(getter1, timer1, expirer1)

							eq(t, actual, mem.GetTimed())

							if !reflect.DeepEqual(expected, actual) {
								t.Fatalf(`
getter0:
	%#[1]v
timer0:
	%#[2]v
expirer0:
	%#[3]v
getter1:
	%#[4]v
timer1:
	%#[5]v
expirer1:
	%#[6]v
expected %[7]T (detailed):
	%#[7]v
actual %[8]T (detailed):
	%#[8]v
expected %[7]T (simple):
	%[7]v
actual %[8]T (simple):
	%[8]v
`, getter0, timer0, expirer0, getter1, timer1, expirer1, expected, actual)
							}
						}
					}
				}
			}
		}
	}
}

func Test_Mem_Dedup_not_expired_no_unnecessary_calls(t *testing.T) {
	for _, getter := range []Getter{nil, failGetter(t)} {
		for _, timer := range []Timer{nil, failTimer(t)} {
			new(Mem).Dedup(getter, timer, BoolExpirer(false))
		}
	}
}

func Test_Mem_Dedup_concurrent_reading(t *testing.T) {
	timed := MakeTimed(`some value`, time.Time{})
	mem := NewMem(timed)
	mem.lock.RLock()

	const count = 8
	var expected []Timed
	actual := make([]Timed, count)

	var wg sync.WaitGroup
	for i := range counter(count) {
		wg.Add(1)

		go func(index int) {
			defer wg.Add(-1)
			actual[index] = mem.Dedup(failGetter(t), failTimer(t), BoolExpirer(false))
		}(i)

		expected = append(expected, timed)
	}

	wg.Wait()

	eq(t, count, len(expected))
	eq(t, count, len(actual))
	eq(t, expected, actual)
}

func Test_Mem_Dedup_waiting_for_writer(t *testing.T) {
	oldTimed := MakeTimed(`old value`, time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC))
	newTimed := MakeTimed(`new value`, time.Date(2, 3, 4, 5, 6, 7, 8, time.UTC))
	mem := NewMem(oldTimed)
	getter := newSlowGetter(newTimed.Get())
	timer := Inst(newTimed.Time)
	writerDone := make(chan struct{})
	readerDone := make(chan struct{})

	go func() {
		defer close(writerDone)
		mem.Dedup(getter, timer, BoolExpirer(true))
	}()

	/**
	What we actually want is to wait precisely until the goroutine above acquires
	the write lock. Unfortunately I'm not aware how to do that with Go mutexes,
	which lack "try lock" functionality. Hence this fragile workaround.
	*/
	time.Sleep(time.Millisecond)

	eq(t, false, isDone(writerDone))

	/**
	As a side effect of the current implementation, a currenly-active writer will
	block even those readers which consider the current value to be non-expired.
	This is unfortunate, but fixing this invites surprising complexity and
	gotchas. Maybe later.
	*/
	go func() {
		defer close(readerDone)
		eq(t, newTimed, mem.Dedup(failGetter(t), failTimer(t), BoolExpirer(false)))
	}()

	// Same workaround as above: wait until the reader is blocked.
	time.Sleep(time.Millisecond)

	eq(t, false, isDone(readerDone))

	// Unblock both. The reader contains the assertion verifying the new value.
	getter.Done()
	eq(t, struct{}{}, <-writerDone)
	eq(t, struct{}{}, <-readerDone)
}

func Benchmark_Mem_refresh(b *testing.B) {
	mem := new(Mem)
	b.ResetTimer()
	for range counter(b.N) {
		benchMemRefresh(mem)
	}
}

//go:noinline
func benchMemRefresh(mem *Mem) {
	// Should regenerate the value every time, using a write lock.
	// All these interface conversions should be zero-alloc.
	// The benchmark should show zero allocs.
	mem.Dedup(GetterFunc(staticGetter), Void{}, BoolExpirer(true))
}
