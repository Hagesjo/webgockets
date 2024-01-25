package websockets

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

var errInvalidFrame = errors.New("invalid frame")

type frame struct {
	Fin bool
	// Whenever these are used, implement them.
	// Rsv1          bool
	// Rsv2          bool
	// Rsv3          bool
	Opcode          OpCode
	CloseStatusCode uint16
	IsMask          bool
	PayloadLength   uint
	Payload         []byte
}

func (f frame) IsControl() bool {
	return f.Opcode > 7
}

func (f frame) ToBytes() ([]byte, error) {
	var bs []byte
	// Fin and op codes.
	b := boolToInt(f.Fin)<<7 | int(f.Opcode)
	bs = append(bs, uint8(b))

	// IsMask and payload.
	if len(f.Payload) > 125 {
		if len(f.Payload) <= 0xFFFF {
			pBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(pBytes, uint16(f.PayloadLength))
			bs = append(bs, uint8(126|boolToInt(f.IsMask)<<7))
			bs = append(bs, pBytes...)
		} else {
			pBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(pBytes, uint64(f.PayloadLength))
			bs = append(bs, uint8(127|boolToInt(f.IsMask)<<7))
			bs = append(bs, pBytes...)
		}
	} else {
		bs = append(bs, uint8(len(f.Payload)|boolToInt(f.IsMask)<<7))
	}

	if f.Opcode == OpcodeConnectionClose {
		pBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(pBytes, uint16(f.CloseStatusCode))
		f.Payload = append(pBytes, f.Payload...)
	}

	if f.IsMask {
		key := make([]byte, 4)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}

		bs = append(bs, key...)

		for i := 0; i < len(f.Payload); i++ {
			bs = append(bs, f.Payload[i]^key[i%len(key)])
		}
	} else {
		bs = append(bs, f.Payload...)
	}

	return bs, nil
}

func readBytes(conn io.ReadWriteCloser, nBytes uint) ([]byte, error) {
	var nRead uint
	var ret []byte
	for nRead != nBytes {
		buf := make([]byte, nBytes-nRead)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}

		ret = append(ret, buf[:n]...)
		nRead += uint(n)
	}

	return ret, nil
}

func parseFrame(conn io.ReadWriteCloser, isPrevFrameFragmented bool) (frame, error) {
	bs, err := readBytes(conn, 2)
	if err != nil {
		return frame{}, err
	}

	var f frame

	// Ensure no RSV is set (byte 5, 6, 7).
	if bs[0]&0b01110000 > 0 {
		return frame{}, fmt.Errorf("rsv byte set: %w", errInvalidFrame)
	}

	// Fin and op codes.
	f.Fin = bs[0]>>7 == 1
	opcode := bs[0] & 0xF
	f.Opcode = OpCode(opcode)

	// Reserved bytes, invalid.
	if f.Opcode > 10 || (f.Opcode >= 3 && f.Opcode <= 7) {
		return frame{}, fmt.Errorf("reserved op code received: %w", errInvalidFrame)
	}

	// IsMask and payload.
	f.IsMask = bs[1]>>7 == 1

	f.PayloadLength = uint(bs[1] & 0b01111111)

	if f.IsControl() {
		// Ensure we don't have fragmented control frames.
		if !f.Fin {
			return frame{}, fmt.Errorf("control frame can't be fragmented: %w", errInvalidFrame)
		}

		if f.PayloadLength > 125 {
			return frame{}, fmt.Errorf("payload is too big (%d) for control frame", f.PayloadLength)
		}
	} else {
		// Ensure we don't have invalid op codes.
		if isPrevFrameFragmented && f.Opcode != 0 {
			return frame{}, fmt.Errorf("fragmented message with op code > 0 received")
		} else if !isPrevFrameFragmented && f.Opcode == 0 {
			return frame{}, fmt.Errorf("continuation frame received but no message to continue")
		}
	}

	if f.PayloadLength == 126 {
		// Payload length is represented by the following 2 bytes.
		bs, err := readBytes(conn, 2)
		if err != nil {
			return frame{}, err
		}

		f.PayloadLength = uint(binary.BigEndian.Uint16(bs))
		f.Payload, err = readBytes(conn, f.PayloadLength)
		if err != nil {
			return frame{}, err
		}
	} else if f.PayloadLength == 127 {
		// Payload length is represented by the following 8 bytes.
		bs, err := readBytes(conn, 8)
		if err != nil {
			return frame{}, err
		}

		f.PayloadLength = uint(binary.BigEndian.Uint64(bs))
		f.Payload, err = readBytes(conn, f.PayloadLength)
		if err != nil {
			return frame{}, err
		}
	} else {
		f.Payload, err = readBytes(conn, f.PayloadLength)
		if err != nil {
			return frame{}, err
		}

		// Validate close frame.
		if f.Opcode == OpcodeConnectionClose {
			if f.PayloadLength == 1 {
				return frame{}, fmt.Errorf("invalid close payload of length 1")
			}

			f.CloseStatusCode = 1000
			if f.PayloadLength >= 2 {
				f.CloseStatusCode = binary.BigEndian.Uint16(f.Payload[:2])
				f.Payload = f.Payload[2:]
				if !utf8.Valid(f.Payload) {
					return frame{}, fmt.Errorf("invalid utf8 in close payload")
				}
			}
		}
	}

	return f, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
