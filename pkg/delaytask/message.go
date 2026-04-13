package delaytask

import "time"

type Message struct {
	Type      string
	Key       string
	Payload   []byte
	ExecuteAt time.Time
}
