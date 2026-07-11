package handler

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type opsCaptureWriterDelegateSpy struct {
	mu          sync.Mutex
	header      http.Header
	calls       []string
	status      int
	size        int
	written     bool
	closeNotify chan bool
	pusher      http.Pusher
	hijackErr   error
}

func newOpsCaptureWriterDelegateSpy() *opsCaptureWriterDelegateSpy {
	return &opsCaptureWriterDelegateSpy{
		header:      make(http.Header),
		status:      http.StatusInternalServerError,
		size:        12,
		written:     true,
		closeNotify: make(chan bool),
		pusher:      opsCaptureWriterPusherSpy{},
		hijackErr:   errors.New("not supported"),
	}
}

func (w *opsCaptureWriterDelegateSpy) called(method string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, method)
}

func (w *opsCaptureWriterDelegateSpy) snapshotCalls() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.calls...)
}

func (w *opsCaptureWriterDelegateSpy) Header() http.Header {
	w.called("Header")
	return w.header
}

func (w *opsCaptureWriterDelegateSpy) Write(p []byte) (int, error) {
	w.called("Write")
	return len(p), nil
}

func (w *opsCaptureWriterDelegateSpy) WriteHeader(int) { w.called("WriteHeader") }
func (w *opsCaptureWriterDelegateSpy) Status() int     { w.called("Status"); return w.status }
func (w *opsCaptureWriterDelegateSpy) Size() int       { w.called("Size"); return w.size }
func (w *opsCaptureWriterDelegateSpy) WriteString(s string) (int, error) {
	w.called("WriteString")
	return len(s), nil
}
func (w *opsCaptureWriterDelegateSpy) Written() bool   { w.called("Written"); return w.written }
func (w *opsCaptureWriterDelegateSpy) WriteHeaderNow() { w.called("WriteHeaderNow") }
func (w *opsCaptureWriterDelegateSpy) Flush()          { w.called("Flush") }
func (w *opsCaptureWriterDelegateSpy) CloseNotify() <-chan bool {
	w.called("CloseNotify")
	return w.closeNotify
}
func (w *opsCaptureWriterDelegateSpy) Pusher() http.Pusher {
	w.called("Pusher")
	return w.pusher
}
func (w *opsCaptureWriterDelegateSpy) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.called("Hijack")
	return nil, nil, w.hijackErr
}

type opsCaptureWriterPusherSpy struct{}

func (opsCaptureWriterPusherSpy) Push(string, *http.PushOptions) error { return nil }

type deterministicOpsCaptureWriterStatePool struct {
	states []*opsCaptureWriterState
}

func (p *deterministicOpsCaptureWriterStatePool) Get() any {
	if len(p.states) == 0 {
		return &opsCaptureWriterState{limit: opsCaptureWriterLimit}
	}
	last := len(p.states) - 1
	state := p.states[last]
	p.states = p.states[:last]
	return state
}

func (p *deterministicOpsCaptureWriterStatePool) Put(value any) {
	state, ok := value.(*opsCaptureWriterState)
	if ok && state != nil {
		p.states = append(p.states, state)
	}
}

type blockingOpsCaptureWriterDelegate struct {
	*opsCaptureWriterDelegateSpy

	writeMu       sync.Mutex
	writes        int
	firstStarted  chan struct{}
	secondStarted chan struct{}
	releaseFirst  chan struct{}
	releaseOnce   sync.Once
}

func newBlockingOpsCaptureWriterDelegate() *blockingOpsCaptureWriterDelegate {
	return &blockingOpsCaptureWriterDelegate{
		opsCaptureWriterDelegateSpy: newOpsCaptureWriterDelegateSpy(),
		firstStarted:                make(chan struct{}),
		secondStarted:               make(chan struct{}),
		releaseFirst:                make(chan struct{}),
	}
}

func (w *blockingOpsCaptureWriterDelegate) Write(p []byte) (int, error) {
	w.called("Write")
	w.writeMu.Lock()
	w.writes++
	call := w.writes
	w.writeMu.Unlock()
	switch call {
	case 1:
		close(w.firstStarted)
		<-w.releaseFirst
	case 2:
		close(w.secondStarted)
	}
	return len(p), nil
}

func (w *blockingOpsCaptureWriterDelegate) unblockFirst() {
	w.releaseOnce.Do(func() { close(w.releaseFirst) })
}

var _ gin.ResponseWriter = (*opsCaptureWriterDelegateSpy)(nil)
var _ gin.ResponseWriter = (*blockingOpsCaptureWriterDelegate)(nil)

func TestOpsCaptureWriter_NilInnerWriter_NoPanic(t *testing.T) {
	w := &opsCaptureWriter{}
	assert.NotPanics(t, func() {
		releaseOpsCaptureWriter(nil)
		releaseOpsCaptureWriter(w)
	})

	assert.NotPanics(t, func() {
		assert.Equal(t, 0, w.Status())
	})
	assert.NotPanics(t, func() {
		assert.Equal(t, -1, w.Size())
	})
	assert.NotPanics(t, func() {
		assert.False(t, w.Written())
	})
	assert.NotPanics(t, func() {
		n, err := w.Write([]byte("test"))
		assert.Equal(t, 0, n)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		n, err := w.WriteString("test")
		assert.Equal(t, 0, n)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		h := w.Header()
		assert.NotNil(t, h)
	})
	assert.NotPanics(t, func() {
		w.WriteHeader(200)
	})
	assert.NotPanics(t, func() {
		w.WriteHeaderNow()
	})
	assert.NotPanics(t, func() {
		w.Flush()
	})
	assert.NotPanics(t, func() {
		conn, rw, err := w.Hijack()
		assert.Nil(t, conn)
		assert.Nil(t, rw)
		assert.Error(t, err)
	})
	assert.NotPanics(t, func() {
		ch := w.CloseNotify()
		assert.NotNil(t, ch)
		select {
		case <-ch:
		default:
			assert.Fail(t, "released CloseNotify channel must already be closed")
		}
	})
	assert.NotPanics(t, func() {
		p := w.Pusher()
		assert.Nil(t, p)
	})
}

func TestOpsCaptureWriter_AllGinResponseWriterMethodsDelegateWhileActive(t *testing.T) {
	delegate := newOpsCaptureWriterDelegateSpy()
	w := acquireOpsCaptureWriter(delegate)
	defer releaseOpsCaptureWriter(w)

	header := w.Header()
	header.Set("X-Ops-Test", "active")
	require.Equal(t, "active", delegate.header.Get("X-Ops-Test"))
	w.WriteHeader(http.StatusBadGateway)
	w.WriteHeaderNow()
	require.Equal(t, delegate.status, w.Status())
	require.Equal(t, delegate.size, w.Size())
	require.Equal(t, delegate.written, w.Written())
	w.Flush()
	conn, readWriter, err := w.Hijack()
	require.Nil(t, conn)
	require.Nil(t, readWriter)
	require.ErrorIs(t, err, delegate.hijackErr)
	require.Equal(t, (<-chan bool)(delegate.closeNotify), w.CloseNotify())
	require.Equal(t, delegate.pusher, w.Pusher())

	n, err := w.Write([]byte("bytes"))
	require.NoError(t, err)
	require.Equal(t, len("bytes"), n)
	n, err = w.WriteString("string")
	require.NoError(t, err)
	require.Equal(t, len("string"), n)
	captured := w.capturedBytes()
	require.Equal(t, []byte("bytesstring"), captured)
	captured[0] = 'X'
	require.Equal(t, []byte("bytesstring"), w.capturedBytes(), "capturedBytes must return a copy")

	calls := delegate.snapshotCalls()
	for _, method := range []string{
		"Header", "WriteHeader", "WriteHeaderNow", "Status", "Size", "Written",
		"Flush", "Hijack", "CloseNotify", "Pusher", "Write", "WriteString",
	} {
		require.Contains(t, calls, method)
	}
}

func TestOpsCaptureWriter_AllDelegatesAreInertAfterRelease(t *testing.T) {
	delegate := newOpsCaptureWriterDelegateSpy()
	w := acquireOpsCaptureWriter(delegate)
	releaseOpsCaptureWriter(w)
	releaseOpsCaptureWriter(w)

	require.Empty(t, w.Header())
	w.WriteHeader(http.StatusNoContent)
	w.WriteHeaderNow()
	w.Flush()
	require.Equal(t, 0, w.Status())
	require.Equal(t, -1, w.Size())
	require.False(t, w.Written())
	n, err := w.Write([]byte("released"))
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = w.WriteString("released")
	require.NoError(t, err)
	require.Zero(t, n)
	conn, rw, err := w.Hijack()
	require.Nil(t, conn)
	require.Nil(t, rw)
	require.Error(t, err)
	requireClosedBoolChannel(t, w.CloseNotify())
	require.Nil(t, w.Pusher())

	require.Empty(t, delegate.snapshotCalls())
}

func TestOpsCaptureWriter_StaleIdentityCannotAccessOrReleasePoolReacquire(t *testing.T) {
	pool := &deterministicOpsCaptureWriterStatePool{}

	staleDelegate := newOpsCaptureWriterDelegateSpy()
	stale := acquireOpsCaptureWriterFromPool(pool, staleDelegate)
	releaseOpsCaptureWriter(stale)

	currentDelegate := newOpsCaptureWriterDelegateSpy()
	current := acquireOpsCaptureWriterFromPool(pool, currentDelegate)
	defer releaseOpsCaptureWriter(current)

	require.NotSame(t, stale, current, "each acquisition must have a distinct writer identity")
	require.Same(t, stale.state, current.state, "test must exercise a pooled-state reacquire")
	_, err := current.WriteString("current-lease")
	require.NoError(t, err)
	require.Equal(t, []byte("current-lease"), current.capturedBytes())
	currentCalls := currentDelegate.snapshotCalls()

	require.Empty(t, stale.Header())
	stale.WriteHeader(http.StatusNoContent)
	stale.WriteHeaderNow()
	stale.Flush()
	require.Zero(t, stale.Status())
	require.Equal(t, -1, stale.Size())
	require.False(t, stale.Written())
	n, err := stale.Write([]byte("stale"))
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = stale.WriteString("stale")
	require.NoError(t, err)
	require.Zero(t, n)
	conn, readWriter, err := stale.Hijack()
	require.Nil(t, conn)
	require.Nil(t, readWriter)
	require.Error(t, err)
	requireClosedBoolChannel(t, stale.CloseNotify())
	require.Nil(t, stale.Pusher())
	require.Nil(t, stale.capturedBytes())
	require.Equal(t, currentCalls, currentDelegate.snapshotCalls(), "stale writer must not reach the reacquired delegate")

	// A stale release must not put the currently active state back into sync.Pool.
	releaseOpsCaptureWriter(stale)
	otherDelegate := newOpsCaptureWriterDelegateSpy()
	other := acquireOpsCaptureWriterFromPool(pool, otherDelegate)
	defer releaseOpsCaptureWriter(other)
	require.NotSame(t, current.state, other.state)

	n, err = current.WriteString("current")
	require.NoError(t, err)
	require.Equal(t, len("current"), n)
	n, err = other.WriteString("other")
	require.NoError(t, err)
	require.Equal(t, len("other"), n)
	require.Contains(t, currentDelegate.snapshotCalls(), "WriteString")
	require.Contains(t, otherDelegate.snapshotCalls(), "WriteString")
}

func TestOpsCaptureWriter_SerializesCaptureAndDelegateWrites(t *testing.T) {
	delegate := newBlockingOpsCaptureWriterDelegate()
	w := acquireOpsCaptureWriter(delegate)
	defer releaseOpsCaptureWriter(w)
	defer delegate.unblockFirst()

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = w.Write([]byte("first"))
	}()
	requireChannelClosed(t, delegate.firstStarted, "first write did not reach delegate")

	secondDone := make(chan struct{})
	secondAttempting := make(chan struct{})
	go func() {
		defer close(secondDone)
		close(secondAttempting)
		_, _ = w.Write([]byte("second"))
	}()
	requireChannelClosed(t, secondAttempting, "second write did not start")
	requireChannelOpen(t, delegate.secondStarted, "second write reached delegate while first write held capture state")

	delegate.unblockFirst()
	requireChannelClosed(t, firstDone, "first write did not complete")
	requireChannelClosed(t, delegate.secondStarted, "second write did not reach delegate after first completed")
	requireChannelClosed(t, secondDone, "second write did not complete")
	require.Equal(t, []byte("firstsecond"), w.capturedBytes())
}

func TestOpsCaptureWriter_ReleaseWaitsForInFlightWrite(t *testing.T) {
	delegate := newBlockingOpsCaptureWriterDelegate()
	w := acquireOpsCaptureWriter(delegate)
	defer releaseOpsCaptureWriter(w)
	defer delegate.unblockFirst()

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		_, _ = w.Write([]byte("in-flight"))
	}()
	requireChannelClosed(t, delegate.firstStarted, "write did not reach delegate")

	releaseDone := make(chan struct{})
	releaseAttempting := make(chan struct{})
	go func() {
		close(releaseAttempting)
		releaseOpsCaptureWriter(w)
		close(releaseDone)
	}()
	requireChannelClosed(t, releaseAttempting, "release did not start")
	requireChannelOpen(t, releaseDone, "release completed while delegate write was in flight")

	delegate.unblockFirst()
	requireChannelClosed(t, writeDone, "write did not complete")
	requireChannelClosed(t, releaseDone, "release did not complete after write")
	require.Nil(t, w.capturedBytes())
}

func TestOpsCaptureWriter_ConcurrentCaptureAccess(t *testing.T) {
	delegate := newOpsCaptureWriterDelegateSpy()
	w := acquireOpsCaptureWriter(delegate)
	defer releaseOpsCaptureWriter(w)

	start := make(chan struct{})
	var writers sync.WaitGroup
	for i := 0; i < 16; i++ {
		writers.Add(1)
		go func(useString bool) {
			defer writers.Done()
			<-start
			for j := 0; j < 32; j++ {
				if useString {
					_, _ = w.WriteString("stream-error")
				} else {
					_, _ = w.Write([]byte("stream-error"))
				}
				_ = w.capturedBytes()
			}
		}(i%2 == 0)
	}
	close(start)
	writers.Wait()

	captured := w.capturedBytes()
	require.NotEmpty(t, captured)
	require.LessOrEqual(t, len(captured), opsCaptureWriterLimit)
}

func requireChannelOpen(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
		require.Fail(t, message)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireChannelClosed(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		require.Fail(t, message)
	}
}

func requireClosedBoolChannel(t *testing.T, ch <-chan bool) {
	t.Helper()
	require.NotNil(t, ch)
	select {
	case <-ch:
	default:
		require.Fail(t, "released CloseNotify channel must already be closed")
	}
}
