package mcp

import (
	"context"
	"io"
	"os"
	"sync"
)

// MockCommander implements Commander for testing without OS processes
type MockCommander struct {
	OnCommand func(ctx context.Context, name string, arg ...string) ExecCommand
}

func (m *MockCommander) Command(ctx context.Context, name string, arg ...string) ExecCommand {
	if m.OnCommand != nil {
		return m.OnCommand(ctx, name, arg...)
	}
	return NewMockExecCommand()
}

// MockExecCommand implements ExecCommand
type MockExecCommand struct {
	stdinReader  *io.PipeReader
	stdinWriter  *io.PipeWriter
	stdoutReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	stderrReader *io.PipeReader
	stderrWriter *io.PipeWriter

	startCalled bool
	waitCalled  bool
	mu          sync.Mutex
	done        chan struct{}
}

func NewMockExecCommand() *MockExecCommand {
	siR, siW := io.Pipe()
	soR, soW := io.Pipe()
	seR, seW := io.Pipe()
	return &MockExecCommand{
		stdinReader:  siR,
		stdinWriter:  siW,
		stdoutReader: soR,
		stdoutWriter: soW,
		stderrReader: seR,
		stderrWriter: seW,
		done:         make(chan struct{}),
	}
}

func (m *MockExecCommand) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
	return nil
}

func (m *MockExecCommand) Wait() error {
	<-m.done
	return nil
}

func (m *MockExecCommand) StdinPipe() (io.WriteCloser, error) {
	return m.stdinWriter, nil
}

func (m *MockExecCommand) StdoutPipe() (io.ReadCloser, error) {
	return m.stdoutReader, nil
}

func (m *MockExecCommand) StderrPipe() (io.ReadCloser, error) {
	return m.stderrReader, nil
}

func (m *MockExecCommand) GetProcess() *os.Process {
	// Return a dummy process with PID 1234
	return &os.Process{Pid: 1234}
}

func (m *MockExecCommand) Exit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

func (m *MockExecCommand) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.waitCalled {
		return
	}
	m.waitCalled = true
	m.stdinWriter.Close()
	m.stdinReader.Close()
	m.stdoutWriter.Close()
	m.stdoutReader.Close()
	m.stderrWriter.Close()
	m.stderrReader.Close()
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}
