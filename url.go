package webgockets

import (
	"fmt"
	"net/url"
)

func validateURL(u string) (*url.URL, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, fmt.Errorf("invalid scheme, expected ws")
	}

	return parsed, nil
}
