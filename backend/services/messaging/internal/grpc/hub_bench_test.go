package grpc

import (
	"fmt"
	"testing"

	pb "ngac-platform/proto/messaging"
)

// BenchmarkBroadcastToChannel measures one instance delivering a message to the
// sessions it holds on one channel: the cost each message adds to a channel that
// many people have open at once.
func BenchmarkBroadcastToChannel(b *testing.B) {
	for _, viewers := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("viewers=%d", viewers), func(b *testing.B) {
			h := NewHub(nil, nil)
			defer h.Close()

			done := make(chan struct{})
			defer close(done)
			// A queue as long as the run: a reader that falls behind is never dropped,
			// so every iteration reaches every viewer.
			for i := range viewers {
				c := &Client{hub: h, tenantID: "t1", userID: fmt.Sprintf("u%d", i), send: make(chan []byte, b.N+1)}
				h.subscribe("ch", "t1", c)
				go func() {
					for {
						select {
						case _, ok := <-c.send:
							if !ok {
								return
							}
						case <-done:
							return
						}
					}
				}()
			}

			msg := &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u0", Content: "hello"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.BroadcastToChannel("ch", msg)
			}
		})
	}
}
