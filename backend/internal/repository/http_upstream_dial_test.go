package repository

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type closeTrackingConn struct {
	net.Conn
	once   sync.Once
	closed chan struct{}
}

func newCloseTrackingConn() *closeTrackingConn {
	return &closeTrackingConn{closed: make(chan struct{})}
}

func (c *closeTrackingConn) Close() error {
	c.once.Do(func() {
		close(c.closed)
	})
	return nil
}

func TestDialResolvedUpstreamIPAddrsClosesLateSuccessfulLoser(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	winner := newCloseTrackingConn()
	loser := newCloseTrackingConn()
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	dial := func(ctx context.Context, network, target string) (net.Conn, error) {
		switch target {
		case "192.0.2.1:443":
			close(firstStarted)
			<-releaseFirst
			return loser, nil
		case "192.0.2.2:443":
			close(secondStarted)
			return winner, nil
		default:
			t.Fatalf("unexpected dial target %s", target)
			return nil, nil
		}
	}

	resultCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := dialResolvedUpstreamIPAddrsWithDial(
			ctx,
			"tcp",
			"443",
			[]net.IPAddr{{IP: net.ParseIP("192.0.2.1")}, {IP: net.ParseIP("192.0.2.2")}},
			time.Millisecond,
			dial,
		)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- conn
	}()

	require.Eventually(t, func() bool {
		select {
		case <-firstStarted:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-secondStarted:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case conn := <-resultCh:
		require.Same(t, winner, conn)
	case <-time.After(time.Second):
		t.Fatal("dial did not return winner")
	}

	close(releaseFirst)
	require.Eventually(t, func() bool {
		select {
		case <-loser.closed:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	select {
	case <-winner.closed:
		t.Fatal("winner connection was closed")
	default:
	}
}
