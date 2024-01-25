package websockets

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	testFraming(t)
	testPingPong(t)
	testReservedBits(t)
	testOpcodes(t)
	testFragmentation(t)
	testClose(t)
	testPerformance(t)
}

func sendReport(t *testing.T) {
	c, err := NewClient("ws://127.0.0.1:9001/updateReports?agent=webgocket")
	if err != nil {
		t.Error(err)
	}

	if err := c.Connect(); err != nil {
		t.Error(err)
	}
}

func testFraming(t *testing.T) {
	for i := 1; i <= 16; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testPingPong(t *testing.T) {
	for i := 17; i <= 27; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testReservedBits(t *testing.T) {
	for i := 28; i <= 34; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testOpcodes(t *testing.T) {
	for i := 35; i <= 44; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testFragmentation(t *testing.T) {
	for i := 45; i <= 64; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testClose(t *testing.T) {
	for i := 65; i <= 101; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func testPerformance(t *testing.T) {
	for i := 102; i <= 156; i++ {
		runTest(t, i)
	}

	sendReport(t)
}

func runTest(t *testing.T, caseN int) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	require := require.New(t)

	logger.Info("Running", "case", caseN)

	c, err := NewClient(fmt.Sprintf("ws://127.0.0.1:9001/runCase?case=%d&agent=webgocket", caseN))
	if err != nil {
		t.Errorf("Failed to create new Client: %s", err)
	}

	if err := c.Connect(); err != nil {
		require.NoError(err)
	}

	for {
		payload, opcode, err := c.read()
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(err)

		if _, err := c.write(payload, opcode); err != nil && errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Error(err)
			break
		}
	}
}
