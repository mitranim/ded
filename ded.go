package ded

import (
	"fmt"
	"sync"
	"time"
)

/*
Something that produces a value. Used as one of the inputs for `Deduper`.
Rules:

	* A nil getter is equivalent to a getter returning nil.

	* Errors are communicated by returning an implementation of `error` or
	  panicking. Both ways are equivalent.

This package is all about deduplicating the TIME and COST of those "get"
operations by using `Mem`.
*/
type Getter interface {
	Get() interface{}
}

/*
Determines a timestamp. Used as one of the inputs for `Deduper`. There are no
requirements and no semantics attached to this timestamp. Timestamps generated
as part of `Dedup` are stored in `Timed`, and interpreted by a user-provided
`Expirer` to determine value expiration.
*/
type Timer interface {
	Time() time.Time
}

/*
Determines the expiration of a particular value. Used as one of the inputs for
`Deduper`. The input to this method may be completely empty, with nil value
and `time.Time{}` timestamp. Many expirers are expected to test only the
timestamp, ignoring the value.
*/
type Expirer interface {
	IsExpired(Timed) bool
}

// Implemented by `*Mem`. Part of the `Omni` interface.
type Deduper interface {
	Dedup(Getter, Timer, Expirer) Timed
}

/*
Combination of all methods relevant for `Mem`, into one value. Used as input to
`Dedup`. This is the recommended way to use this package. Define your own type
that embeds `Mem`, `NowTimer` and one of the expirer types, and define your own
`.Get` method to complete this interface. See the `Dedup` example in the
readme.
*/
type Omni interface {
	Getter
	Timer
	Expirer
	Deduper
}

/*
Creates an instance of `Mem` with the given value and time. Defined mostly for
tests. User code shouldn't have to instantiate `Mem` manually, because the zero
value is ready to use.
*/
func NewMem(val Timed) *Mem { return &Mem{val: val} }

/*
Tool for deduplicating data-fetching operations. The zero value is ready to use,
but must not be copied (use it by pointer). Conceptually, this is something
like `atomic.Value<Timed>`, with the added ability to synchronize readers with
writers, and to avoid generating new states when the value is not expired.

Intended for simultaneous use by many concurrent readers. As such, all methods
of `*Mem` are concurrency-safe.
*/
type Mem struct {
	lock sync.RWMutex
	val  Timed
}

/*
Shorthand for `.GetTimed().Get()`. Returns the currently-cached inner value,
which is initially nil. If an error is currently cached, panics with that
error. If a writer is currently generating a new value, this blocks until the
writer is finished, returning the new value.
*/
func (self *Mem) Get() interface{} { return self.GetTimed().Get() }

/*
Returns the currently-cached state. Initially this returns the zero value
`Timed{}`. If a writer is currently generating a new value, this blocks until
the writer is finished, returning the new state.
*/
func (self *Mem) GetTimed() Timed {
	self.lock.RLock()
	defer self.lock.RUnlock()
	return self.val
}

// Replaces the cached state with the provided state.
func (self *Mem) SetTimed(val Timed) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.val = val
}

// Zeroes the state, resetting it to `Timed{}`.
func (self *Mem) Zero() { self.SetTimed(Timed{}) }

/*
Main API of this package. Uses the provided expirer to determine the freshness
of the currently-stored value. If fresh enough, returns the value as-is.
Otherwise, uses the provided getter and timer to generate a new value with its
timestamp, and returns the new result.

Uses `sync.RWMutex` to synchronize access. Concurrent readers don't block each
other. Writers block everyone else. The provided getter is assumed to be slow
and expensive. Only the writer holding the write lock is allowed to regenerate
the value by calling the getter.
*/
func (self *Mem) Dedup(get Getter, time Timer, exp Expirer) Timed {
	val := self.GetTimed()
	if !IsExpired(exp, val) {
		return val
	}

	// When multiple goroutines simultaneously try to acquire this lock, one
	// succeeds immediately and proceeds to make a new value, while others
	// succeed later.
	self.lock.Lock()
	defer self.lock.Unlock()

	// We must re-check expiration, because while we were acquiring the write
	// lock, countless other writers may have done it first, regenerating the
	// value.
	val = self.val
	if !IsExpired(exp, val) {
		return val
	}

	self.val.SetGetter(get)
	self.val.SetTimer(time)
	return self.val
}

// Implement `fmt.GoStringer` for debug purposes.
func (self *Mem) GoString() string {
	return fmt.Sprintf(`ded.NewMem(%#v)`, self.GetTimed())
}

// Same as `val.Get()` but nil-safe. Fallback output is nil.
func Get(val Getter) interface{} {
	if val != nil {
		return val
	}
	return nil
}

// Same as `val.Get()` but nil-safe. Fallback output is `time.Time{}`.
func Time(val Timer) time.Time {
	if val != nil {
		return val.Time()
	}
	return time.Time{}
}

// Same as `exp.IsExpired(timed)` but nil-safe. Fallback output is `true`,
// meaning that if no expirer is provided, everything is considered expired.
func IsExpired(exp Expirer, timed Timed) bool {
	return exp == nil || exp.IsExpired(timed)
}

/*
Same as `val.Dedup(val, val, val)`. Shorthand for types that combine all
relevant methods into one by embedding `Mem` and other types such as `NowTimer`
and `ExpireMinute`. This approach is the recommended way to use this package.
See the attached example.
*/
func Dedup(val Omni) Timed {
	if val != nil {
		return val.Dedup(val, val, val)
	}
	return Timed{}
}

/*
Represents either value or error. If the inner value implements `error`,
unwrapping with `.Get()` will panic. Supports "set"-style methods that catch
and store panics. Currently uses only one `interface{}` field to avoid wasting
memory. The representation may change in future versions.
*/
type Either [1]interface{}

/*
If the inner value implements `error`, panics with that error. Otherwise,
returns the inner value as-is.
*/
func (self Either) Get() interface{} {
	val, err := self.Unwrap()
	if err != nil {
		panic(err)
	}
	return val
}

/*
If the inner value implements `error`, returns `(nil, err)`. Otherwise, returns
`(val, nil)`.
*/
func (self Either) Unwrap() (interface{}, error) {
	val := self[0]
	err, _ := val.(error)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Replaces the inner value.
func (self *Either) Set(val interface{}) { self[0] = val }

/*
Replaces the inner value by calling the provided getter. Nil getter is ok and
considered to have nil value. If the getter panics, the panic is caught and
stored as inner value. Later, attempting to `.Get()` a caught error will
panic.
*/
func (self *Either) SetGetter(val Getter) {
	if val == nil {
		self.Set(nil)
		return
	}

	defer self.rec()
	self.Set(val.Get())
}

// Implement `fmt.GoStringer` for debug purposes.
func (self Either) GoString() string {
	return fmt.Sprintf(`ded.Either{%#v}`, self[0])
}

// Must be deferred.
func (self *Either) rec() {
	val := recover()
	if val != nil {
		self.Set(val)
	}
}

// Shortcut for constructing `Timed`.
func MakeTimed(val interface{}, inst time.Time) Timed {
	return Timed{Either{val}, inst}
}

/*
Combination of an arbitrary value and a timestamp. Produced and stored by `Mem`.
Used as input to `Expirer`. The inner value is stored in `.Either` and accessed
by calling `.Get`; see the `Either` docs. The timestamp has no inherent
semantics; it's produced AND evaluated by user-provided implementations of
`Timer` and `Expirer`.
*/
type Timed struct {
	Either
	Time time.Time
}

/*
Replaces the timestamp by calling `val.Time()`. Nil timer is ok, equivalent to
`time.Time{}`. If the timer panics, the resulting panic replaces the inner
value stored in `.Either`, while the timestamp is unaffected.
*/
func (self *Timed) SetTimer(val Timer) {
	if val == nil {
		self.Time = time.Time{}
		return
	}

	defer self.rec()
	self.Time = val.Time()
}

// Implement `fmt.GoStringer` for debug purposes.
func (self Timed) GoString() string {
	return fmt.Sprintf(`ded.MakeTimed(%#v, %#v)`, self.Either[0], self.Time)
}

/*
Implements `Expirer` like this: `time.Now() > (input + self)`. When duration is
negative, only future timestamps can pass.

On 64-bit machines, interface conversion `Expirer(Duration(val))` doesn't
seem to allocate (tested in Go 1.17). Passing it inline is just as good as
allocating statically.
*/
type Duration time.Duration

var _ = Expirer(Duration(0))

// Free cast to `time.Duration`. Slightly shorter to type.
func (self Duration) Duration() time.Duration { return time.Duration(self) }

// Implement `Expirer`. See the description on the type.
func (self Duration) IsExpired(val Timed) bool {
	return time.Now().After(val.Time.Add(self.Duration()))
}

/*
Short for "instant".
Typedef for `time.Time`.
Implements `Timer` by returning itself.
Implements `Expirer` like this: `input > self`.
*/
type Inst time.Time

var (
	_ = Timer(Inst(time.Time{}))
	_ = Expirer(Inst(time.Time{}))
)

// Implement `Timer` by freely casting itself to `time.Time`.
func (self Inst) Time() time.Time { return time.Time(self) }

// Implement `Expirer` like this: `input > self`.
func (self Inst) IsExpired(val Timed) bool { return val.Time.After(self.Time()) }

// Implement `fmt.Stringer` for debug purposes.
func (self Inst) String() string { return self.Time().String() }

// Implement `fmt.GoStringer` for debug purposes.
func (self Inst) GoString() string { return fmt.Sprintf(`ded.Inst(%#v)`, self.Time()) }

/*
Implements `Timer` by calling `time.Now()`. This type is zero-sized, and can be
embedded in other types for free to add this method, like a mixin, or cast to
an interface without allocating.
*/
type NowTimer struct{}

var _ = Timer(NowTimer{})

// Implement `Timer` by returning `time.Now()`.
func (NowTimer) Time() time.Time { return time.Now() }

/*
Implements `Expirer` like this: `time.Now() > input`. This type is zero-sized,
and can be embedded in other types for free to add this method, like a mixin,
or cast to an interface without allocating.
*/
type NowExpirer struct{}

var _ = Expirer(NowExpirer{})

// Implement `Expirer` like this: `now > input`.
func (NowExpirer) IsExpired(val Timed) bool { return time.Now().After(val.Time) }

/*
Implements `Getter` by calling self. Returns nil if func is nil.
Interface conversion `AnyInterface(GetterFunc(someFunc))` is zero-alloc.
*/
type GetterFunc func() interface{}

var _ = Getter(GetterFunc(nil))

// Implement `Getter` by calling itself. Returns nil if func is nil.
func (self GetterFunc) Get() interface{} {
	if self != nil {
		return self()
	}
	return nil
}

/*
Implements `Timer` by calling self. Returns `time.Time{}` if func is nil.
Interface conversion `AnyInterface(TimerFunc(someFunc))` is zero-alloc.
*/
type TimerFunc func() time.Time

var _ = Timer(TimerFunc(nil))

// Implement `Timer` by calling itself. Returns `time.Time{}` if func is nil.
func (self TimerFunc) Time() time.Time {
	if self != nil {
		return self()
	}
	return time.Time{}
}

/*
Implements `Getter` by returning nil.
Implements `Timer` by returning `time.Time{}`.
Implements `Expirer` by returning true.
This type is zero-sized, and can be embedded in other types for free to add
methods, like a mixin, or cast to an interface without allocating.
*/
type Void struct{}

var (
	_ = Getter(Void{})
	_ = Timer(Void{})
	_ = Expirer(Void{})
)

// Implement `Getter` by returning nil.
func (Void) Get() interface{} { return nil }

// Implement `Timer` by returning `time.Time{}`.
func (Void) Time() time.Time { return time.Time{} }

// Implement `Expirer` by returning true (always expire).
func (Void) IsExpired(Timed) bool { return true }

/*
Implements `Expirer` by requiring that a given timestamp is no more than a
second old. This type is zero-sized, and can be embedded in other types for
free to add this method, like a mixin.
*/
type ExpireSecond struct{}

// Implement `Expirer` like this: `now > (input + second)`.
func (ExpireSecond) IsExpired(val Timed) bool {
	return Duration(time.Second).IsExpired(val)
}

/*
Implements `Expirer` by requiring that a given timestamp is no more than a
minute old. This type is zero-sized, and can be embedded in other types for
free to add this method, like a mixin.
*/
type ExpireMinute struct{}

// Implement `Expirer` like this: `now > (input + minute)`.
func (ExpireMinute) IsExpired(val Timed) bool {
	return Duration(time.Minute).IsExpired(val)
}

/*
Implements `Expirer` by requiring that a given timestamp is no more than an hour
old. This type is zero-sized, and can be embedded in other types for free to
add this method, like a mixin.
*/
type ExpireHour struct{}

// Implement `Expirer` like this: `now > (input + hour)`.
func (ExpireHour) IsExpired(val Timed) bool {
	return Duration(time.Hour).IsExpired(val)
}

/*
Implements `Expirer` by requiring that a given timestamp is no more than a day
old. This type is zero-sized, and can be embedded in other types for free to
add this method, like a mixin.
*/
type ExpireDay struct{}

// Implement `Expirer` like this: `now > (input + day)`.
func (ExpireDay) IsExpired(val Timed) bool {
	return Duration(time.Hour * 24).IsExpired(val)
}
