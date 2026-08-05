package client

import (
	"bufio"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
}

func ParseSSEStream(r io.Reader, fn func(SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	var event SSEEvent
	hasData := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if hasData {
				if err := fn(event); err != nil {
					return err
				}
			}
			event = SSEEvent{}
			hasData = false
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			event.Data = line[6:]
			hasData = true
		} else if line == "data:" {
			event.Data = ""
			hasData = true
		} else if strings.HasPrefix(line, "event: ") {
			event.Event = line[7:]
			hasData = true
		}
	}

	return scanner.Err()
}
