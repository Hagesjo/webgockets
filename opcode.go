package webgockets

import (
	"fmt"
	"strconv"
)

type OpCode int

func (o OpCode) String() string {
	switch o {
	case 0:
		return "unknown"
	case 1:
		return "text"
	case 2:
		return "binary"
	case 3, 4, 5, 6, 7:
		return fmt.Sprintf("reserved non-control %d", o-2)
	case 8:
		return "conn closed"
	case 9:
		return "ping"
	case 10:
		return "pong"
	case 11, 12, 13, 14, 15:
		return fmt.Sprintf("reserved control %d", o-10)
	default:
		return strconv.Itoa(int(o))
	}
}

const (
	OpcodeContinuationFrame OpCode = iota
	OpcodeTextFrame
	OpcodeBinaryFrame
	OpcodeReservedNonControl1
	OpcodeReservedNonControl2
	OpcodeReservedNonControl3
	OpcodeReservedNonControl4
	OpcodeReservedNonControl5
	OpcodeConnectionClose
	OpcodePing
	OpcodePong
	OpcodeReservedControl1
	OpcodeReservedControl2
	OpcodeReservedControl3
	OpcodeReservedControl4
	OpcodeReservedControl5
)
