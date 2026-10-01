package model

// Ring is a fixed-capacity buffer that overwrites its oldest value when full.
// It is not safe for concurrent use: one owner goroutine pushes and reads.
type Ring[T any] struct {
	buf  []T
	next int
	n    int
}

// NewRing returns an empty ring holding at most capacity values. It panics if
// capacity < 1.
func NewRing[T any](capacity int) *Ring[T] {
	if capacity < 1 {
		panic("model: ring capacity must be >= 1")
	}
	return &Ring[T]{buf: make([]T, capacity)}
}

// Push appends v, overwriting the oldest value when the ring is full.
func (r *Ring[T]) Push(v T) {
	r.buf[r.next] = v
	r.next = (r.next + 1) % len(r.buf)
	if r.n < len(r.buf) {
		r.n++
	}
}

// Len returns the number of values held.
func (r *Ring[T]) Len() int { return r.n }

// Cap returns the fixed capacity.
func (r *Ring[T]) Cap() int { return len(r.buf) }

// Values returns a new slice of the held values, oldest first.
func (r *Ring[T]) Values() []T {
	out := make([]T, 0, r.n)
	start := (r.next - r.n + len(r.buf)) % len(r.buf)
	for i := 0; i < r.n; i++ {
		out = append(out, r.buf[(start+i)%len(r.buf)])
	}
	return out
}
