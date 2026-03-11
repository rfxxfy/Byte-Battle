package ws

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHub_BroadcastReachesAllClients(t *testing.T) {
	h := NewHub()
	c1 := &Client{send: make(chan []byte, 8)}
	c2 := &Client{send: make(chan []byte, 8)}

	h.Join(1, c1)
	h.Join(1, c2)

	msg := []byte("hello")
	h.Broadcast(1, msg)

	assert.Equal(t, msg, <-c1.send)
	assert.Equal(t, msg, <-c2.send)
}

func TestHub_LeaveStopsBroadcast(t *testing.T) {
	h := NewHub()
	c := &Client{send: make(chan []byte, 8)}

	h.Join(1, c)
	h.Leave(1, c)
	h.Broadcast(1, []byte("hi"))

	select {
	case <-c.send:
		t.Fatal("client should not receive message after Leave")
	default:
	}
}

func TestHub_BroadcastNoRoom(t *testing.T) {
	h := NewHub()
	// should not panic on nonexistent room
	h.Broadcast(999, []byte("hi"))
}

func TestHub_BroadcastFullBufferDoesNotBlock(t *testing.T) {
	h := NewHub()
	c := &Client{send: make(chan []byte, 1)}
	h.Join(1, c)

	h.Broadcast(1, []byte("msg1")) // fills buffer
	h.Broadcast(1, []byte("msg2")) // dropped, must not block

	assert.Equal(t, []byte("msg1"), <-c.send)
	select {
	case <-c.send:
		t.Fatal("second message should have been dropped")
	default:
	}
}

func TestHub_ConcurrentJoinLeave(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(id int32) {
			defer wg.Done()
			c := &Client{send: make(chan []byte, 64)}
			h.Join(id, c)
			h.Broadcast(id, []byte("msg"))
			h.Leave(id, c)
		}(int32(i))
	}

	wg.Wait()
}
