package handlers

import (
	"fmt"

	"learning-go/internal/events"
)

type LoggerHandler struct{}

func NewLoggerHandler() *LoggerHandler {
	return &LoggerHandler{}
}

func (h *LoggerHandler) Handle(event events.Event) error {
	fmt.Printf("[EVENT] %s: %+v\n", event.Type(), event)
	return nil
}
