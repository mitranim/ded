package ded

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func eq(t testing.TB, exp, act interface{}) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		fatalEq(t, ``, exp, act)
	}
}

func fatalEq(t testing.TB, prefix string, exp, act interface{}) {
	t.Helper()

	t.Fatalf(prefix+`
expected (detailed):
	%#[1]v
actual (detailed):
	%#[2]v
expected (simple):
	%[1]v
actual (simple):
	%[2]v
`, exp, act)
}

func panics(t testing.TB, val interface{}, fun func()) {
	t.Helper()
	defer recPanics(t, val, fun)
	fun()
}

func recPanics(t testing.TB, exp interface{}, fun interface{}) {
	t.Helper()
	val := recover()

	if val == nil {
		t.Fatalf(`expected %v to panic, found nil (nil panic is undetectable)`, funcName(fun))
	}

	if !reflect.DeepEqual(exp, val) {
		prefix := fmt.Sprintf(`%v panicked with the wrong value`, funcName(fun))
		fatalEq(t, prefix, exp, val)
	}
}

func funcName(val interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(val).Pointer()).Name()
}

func testErr() error { return fmt.Errorf(`some error`) }

func testGet(t testing.TB, val interface{}, src Getter) {
	err, _ := val.(error)
	if err != nil {
		panics(t, err, func() { src.Get() })
		panics(t, err, func() { src.Get() })
		return
	}

	eq(t, val, src.Get())
	eq(t, val, src.Get())
}

func counter(val int) []struct{} { return make([]struct{}, val) }

type BoolExpirer bool

func (self BoolExpirer) IsExpired(Timed) bool { return bool(self) }

var (
	testVals = []interface{}{nil, 10, `val`, testErr()}

	testTimes = []time.Time{time.Time{}, time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC)}

	testGetters = append([]Getter{nil}, valsToGetters(testVals)...)

	testTimers = append([]Timer{nil, Void{}}, timesToTimers(testTimes)...)

	testExpirers = append(
		[]Expirer{nil, Void{}, NowExpirer{}, BoolExpirer(false), BoolExpirer(true)},
		timesToExpirers(testTimes)...,
	)
)

func valsToGetters(vals []interface{}) []Getter {
	if vals == nil {
		return nil
	}

	out := make([]Getter, len(vals))
	for i, val := range vals {
		out[i] = Either{val}
	}
	return out
}

func timesToTimers(vals []time.Time) []Timer {
	if vals == nil {
		return nil
	}

	out := make([]Timer, len(vals))
	for i, val := range vals {
		out[i] = Inst(val)
	}
	return out
}

func timesToExpirers(vals []time.Time) []Expirer {
	if vals == nil {
		return nil
	}

	out := make([]Expirer, len(vals))
	for i, val := range vals {
		out[i] = Inst(val)
	}
	return out
}

func failGetter(t testing.TB) Getter {
	return GetterFunc(func() interface{} { t.Fail(); return nil })
}

func failTimer(t testing.TB) Timer {
	return TimerFunc(func() time.Time { t.Fail(); return time.Time{} })
}

func newSlowGetter(val interface{}) *slowGetter {
	var out slowGetter
	out.Add(1)
	out.Store(val)
	return &out
}

type slowGetter struct {
	sync.WaitGroup
	atomic.Value
}

func (self *slowGetter) Get() interface{} {
	self.Wait()
	return Either{self.Load()}.Get()
}

func isDone(val <-chan struct{}) bool {
	select {
	case <-val:
		return true
	default:
		return false
	}
}

// In Go 1.17, constant-to-interface doesn't alloc.
func staticGetter() interface{} { return `some val` }
