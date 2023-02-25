package rpc

import (
	"encoding/gob"
	"fmt"
)

func init() {
	gob.Register(&Message{})
}

type Message struct {
	ID       string
	Method   string
	Body     []byte
	Metadata map[string]any
	Error    error
}

var ErrUnsupportedMethod = fmt.Errorf("unsupported method")
