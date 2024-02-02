package webgockets

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidJSON = errors.New("invalid json")

// ReadJSON is a convenience function to unmarshal a client read into a json struct.
// Ideally, this should be a method to the client, but due to go limitations with generic methods, we can't do that for now.
func ReadJSON[T any](client *Client) (*T, error) {
	bs, err := client.Read()
	if err != nil {
		return nil, err
	}

	var ret T
	if err := json.Unmarshal(bs, &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return &ret, nil
}
