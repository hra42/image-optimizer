package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/sse"
)

// Progress handles GET /progress/:jobId as a Server-Sent Events stream. It
// replays any events emitted before the client connected, then streams live
// per-preset events until the job finishes or the client disconnects. The sse
// middleware owns the wire format, headers, heartbeats, and flushing.
func Progress(store *Store) fiber.Handler {
	return sse.New(sse.Config{
		Handler: func(c fiber.Ctx, stream *sse.Stream) error {
			jobID := c.Params("jobId")

			history, ch, done, ok := store.Subscribe(jobID)
			if !ok {
				// Unknown job: tell the client and close.
				return stream.Event(sse.Event{
					Data: progressEvent{Job: jobID, Status: "error"},
				})
			}

			// Replay everything emitted before this subscription attached.
			for _, ev := range history {
				if err := stream.Event(sse.Event{Data: ev}); err != nil {
					return err
				}
			}
			if done {
				// Job already finished; history held the terminal event.
				return nil
			}

			defer store.Unsubscribe(jobID, ch)

			for {
				select {
				case ev, open := <-ch:
					if !open {
						// Finish/Fail closed the channel — the stream is done.
						return nil
					}
					if err := stream.Event(sse.Event{Data: ev}); err != nil {
						return err // write failure / client gone
					}
				case <-stream.Context().Done():
					return stream.Context().Err() // client disconnected
				}
			}
		},
	})
}
