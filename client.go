package webgockets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type Client struct {
	c       *http.Client
	wsURL   *url.URL
	httpURL *url.URL
	socket  io.ReadWriteCloser
	frames  chan frame
	// fragmented indicates that the current frames being received is fragmented.
	fragmented bool
}

// NewClient validates the wsURL and creates a new Client.
func NewClient(wsURL string) (*Client, error) {
	u, err := validateURL(wsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	httpURL := u
	if u.Scheme == "ws" {
		httpURL.Scheme = "http"
	} else {
		httpURL.Scheme = "https"
	}

	client := &Client{
		c:       http.DefaultClient,
		wsURL:   u,
		httpURL: httpURL,
		frames:  make(chan frame, 100), // TODO: Can most likely just have this as blocking.
	}

	return client, nil
}

// Connect handshakes and starts receiving messages.
func (c *Client) Connect() error {
	w, err := c.handshake()
	if err != nil {
		return fmt.Errorf("failed to handshake: %w", err)
	}

	c.socket = w

	go func() {
		err = c.readHandler()
	}()

	return err
}

func (c *Client) handshake() (io.ReadWriteCloser, error) {
	req, err := http.NewRequest("GET", c.httpURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Connection", "Upgrade")

	key, err := handshakeKeyRequest()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Sec-Websocket-Key", key)
	req.Header.Add("Sec-WebSocket-Protocol", "chat, superchat")
	req.Header.Add("Sec-WebSocket-Version", "13")

	resp, err := c.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	if resp.StatusCode != 101 {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	expectedAcceptKey, err := handshakeAcceptKey(key)
	if err != nil {
		return nil, err
	}

	expectedHeaders := map[string]string{
		"Upgrade":    "websocket",
		"Connection": "upgrade",
	}

	for header, value := range expectedHeaders {
		h := strings.ToLower(resp.Header.Get(header))
		if h != value {
			return nil, fmt.Errorf("unexpected header %s: %s, wanted %s", header, h, value)
		}
	}

	// Case sensitive.
	expectedExactHeaders := map[string]string{
		"Sec-Websocket-Accept": expectedAcceptKey,
	}

	for header, value := range expectedExactHeaders {
		h := resp.Header.Get(header)
		if h != value {
			return nil, fmt.Errorf("unexpected header %s: %s, wanted %s", header, h, value)
		}
	}

	w, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		return nil, fmt.Errorf("wasn't able to make body a writer")
	}

	return w, nil
}

// Read reads a non-fragmented frame, or concatenates the payload of multiple fragmented frames.
func (c *Client) Read() ([]byte, error) {
	payload, _, err := c.read()
	return payload, err
}

func (c *Client) ReadString() (string, error) {
	bs, err := c.Read()
	return string(bs), err
}

func (c *Client) read() ([]byte, OpCode, error) {
	frame, err := c.getFrame()
	if err != nil {
		return nil, 0, err
	}

	payload := frame.Payload
	opcode := frame.Opcode

	for !frame.Fin {
		frame, err = c.getFrame()
		if err != nil {
			return nil, 0, err
		}

		payload = append(payload, frame.Payload...)
	}

	return payload, opcode, nil
}

func (c *Client) getFrame() (frame, error) {
	f, ok := <-c.frames
	if !ok {
		return frame{}, &ErrClose{}
	}

	for f.IsControl() {
		if err := c.controlHandler(f); err != nil {
			return frame{}, err
		}

		if f.Opcode == OpcodeConnectionClose {
			return frame{}, &ErrClose{Code: f.CloseStatusCode, Payload: string(f.Payload)}
		}

		f, ok = <-c.frames
		if !ok {
			return frame{}, &ErrClose{}
		}
	}

	return f, nil
}

func (c *Client) readHandler() error {
	for {
		frame, err := c.readFrame()
		if err != nil {
			return c.Close()
		}

		if frame.IsControl() {
			if err := c.controlHandler(frame); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to handle control frame: %w", err)
			}

			continue
		}

		c.frames <- frame
	}
}

func (c *Client) readFrame() (frame, error) {
	f, err := parseFrame(c.socket, c.fragmented)
	if errors.Is(err, errInvalidFrame) {
		return frame{}, errInvalidFrame
	} else if err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return frame{}, io.EOF
		}

		return frame{}, err
	}

	// Fragmentation done if Fin is set.
	if !f.IsControl() {
		c.fragmented = !f.Fin
	}

	return f, nil
}

type ErrClose struct {
	Code    uint16
	Payload string
}

func (e *ErrClose) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Payload)
}

func (c *Client) controlHandler(frame frame) error {
	switch frame.Opcode {
	case OpcodePong:
		// Do nothing.
	case OpcodePing:
		return c.sendPong(frame.Payload)
	case OpcodeConnectionClose:
		return c.sendClose(frame.CloseStatusCode)
	default:
		return fmt.Errorf("unknown op code %d", frame.Opcode)
	}

	return nil
}

func (c *Client) sendPong(payload []byte) error {
	frame := frame{
		Fin:     true,
		Opcode:  OpcodePong,
		IsMask:  true,
		Payload: payload,
	}

	bs, err := frame.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to make frame bytes: %w", err)
	}

	if _, err := c.socket.Write(bs); err != nil {
		return fmt.Errorf("failed to write bytes: %w", err)
	}

	return nil
}

var validBasicCloseCodes = []uint16{1000, 1001, 1002, 1003, 1007, 1008, 1009, 1010, 1011}

func isValidCloseCode(code uint16) bool {
	if slices.Contains(validBasicCloseCodes, code) {
		return true
	}

	return code >= 3000 && code <= 4999
}

func (c *Client) sendClose(statusCode uint16) error {
	if !isValidCloseCode(statusCode) {
		statusCode = 1002
	}

	frame := frame{
		Fin:             true,
		Opcode:          OpcodeConnectionClose,
		CloseStatusCode: statusCode,
		IsMask:          true,
		Payload:         []byte("Going away"),
	}

	bs, err := frame.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to make frame bytes: %w", err)
	}

	if _, err := c.socket.Write(bs); err != nil {
		return fmt.Errorf("failed to write bytes: %w", err)
	}

	return nil
}

func (c *Client) WriteString(str string) (int, error) {
	return c.Write([]byte(str))
}

// Write writes a text frame.
func (c *Client) Write(bs []byte) (int, error) {
	return c.write(bs, OpcodeTextFrame)
}

// WriteJSON writes a text frame with marshaled bytes from the given json struct.
func (c *Client) WriteJSON(jsonData any) (int, error) {
	bs, err := json.Marshal(jsonData)
	if err != nil {
		return 0, fmt.Errorf("invalid json: %w", err)
	}

	return c.Write(bs)
}

// WriteBinary writes a binary frame
func (c *Client) WriteBinary(bs []byte) (int, error) {
	return c.write(bs, OpcodeBinaryFrame)
}

func (c *Client) write(bs []byte, opCode OpCode) (int, error) {
	if opCode != OpcodeBinaryFrame && opCode != OpcodeTextFrame {
		return 0, fmt.Errorf("invalid op code")
	}

	frameBs, err := frame{
		Fin:           true,
		Opcode:        opCode,
		IsMask:        true,
		PayloadLength: uint(len(bs)),
		Payload:       bs,
	}.ToBytes()
	if err != nil {
		return 0, fmt.Errorf("failed to make frame bytes: %w", err)
	}

	n, err := c.socket.Write(frameBs)
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		return n, io.EOF
	}

	return 0, err
}

func (c *Client) Close() error {
	if err := c.socket.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	close(c.frames)

	return nil
}
