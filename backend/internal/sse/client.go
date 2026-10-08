package sse

import (
	"fmt"
	"net/http"
)

type Client struct {
	hub     *Hub
	w       http.ResponseWriter
	send    chan []byte
	queueID int64
}

func NewClient(
	hub *Hub,
	w http.ResponseWriter,
	queueID int64,
) *Client {
	return &Client{
		hub:     hub,
		w:       w,
		send:    make(chan []byte, 256),
		queueID: queueID,
	}
}

func (c *Client) Serve(r *http.Request) {
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			fmt.Fprintf(c.w, "data: %s\n\n", message)
			flusher, ok := c.w.(http.Flusher)
			if ok {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}
