package mcpproxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

type ProxyRunnerFunc func(listen string, externalURL string, commandArgs []string) error

// StdioServer handles execution and relaying of stdio MCP subprocesses
type StdioServer struct {
	cmd          *exec.Cmd
	stdinPipe    io.WriteCloser
	stdoutReader *bufio.Reader
	stderrReader io.ReadCloser
	mu           sync.Mutex
}

func NewStdioServer(command []string) (*StdioServer, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("no command provided for stdio server")
	}

	cmd := exec.Command(command[0], command[1:]...)
	// Ensure Python subprocesses run in unbuffered stdout mode
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1", "FORCE_COLOR=0")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	server := &StdioServer{
		cmd:          cmd,
		stdinPipe:    stdin,
		stdoutReader: bufio.NewReaderSize(stdout, 10*1024*1024), // 10MB line buffer size
		stderrReader: stderr,
	}

	// Consume stderr concurrently to prevent pipe buffer deadlock
	go server.logStderr()

	return server, nil
}

func (s *StdioServer) logStderr() {
	scanner := bufio.NewScanner(s.stderrReader)
	for scanner.Scan() {
		log.Printf("[MCP Subprocess Stderr] %s", scanner.Text())
	}
}

// SendJSONRPC sends a JSON-RPC request over stdin and reads a line-delimited response from stdout.
// Non-JSON noise lines emitted to stdout are skipped and logged to prevent stream corruption.
func (s *StdioServer) SendJSONRPC(ctx context.Context, payload []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure payload ends with newline
	if !bytes.HasSuffix(payload, []byte("\n")) {
		payload = append(payload, '\n')
	}

	if _, err := s.stdinPipe.Write(payload); err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	for {
		line, err := s.stdoutReader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				line = bytes.TrimSpace(line)
				if isJSONRPC(line) {
					return line, nil
				}
			}
			return nil, fmt.Errorf("error reading stdout from downstream stdio server: %w", err)
		}

		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		// Filter out non-JSON output (e.g. package installation logs or print statements)
		if !isJSONRPC(trimmed) {
			log.Printf("[MCP Subprocess Stdout Non-JSON Noise] %s", string(trimmed))
			continue
		}

		return trimmed, nil
	}
}

func isJSONRPC(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	first := b[0]
	return first == '{' || first == '['
}

