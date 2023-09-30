package ptymaster

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	ptyhandler "github.com/creack/pty"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	Rows = 32
	Cols = 140
)

type PTYHandler interface {
	Write(data []byte) (int, error)
	Refresh()
}

// PTYMaster defines a PTY Master whih will encapsulate the command we want to run, and provide simple
// access to the command, to write and read IO, but also to control the window size.
type PTYMaster struct {
	ptyFile           *os.File
	command           *exec.Cmd
	terminalInitState *terminal.State
}

func New() *PTYMaster {
	return &PTYMaster{}
}

func (m *PTYMaster) Start(command string, args []string, envVars []string) (pid int, err error) {
	m.command = exec.Command(command, args...)
	m.command.Env = envVars

	pty, tty, err := ptyhandler.Open()
	if err != nil {
		return 0, fmt.Errorf("could not open pty: %w", err)
	}
	defer func(tty *os.File) {
		err := tty.Close()
		if err != nil {
			log.WithError(err).Error("could not close tty")
		}
	}(tty)

	if err := ptyhandler.Setsize(pty, &ptyhandler.Winsize{
		Rows: uint16(Rows),
		Cols: uint16(Cols),
	}); err != nil {
		errClose := pty.Close()
		if err != nil {
			return 0, fmt.Errorf("could not set size: %w: could not close pty: %w", err, errClose)
		}
		return 0, fmt.Errorf("could not set pty size: %w", err)
	}

	if m.command.Stdout == nil {
		m.command.Stdout = tty
	}

	if m.command.Stderr == nil {
		m.command.Stderr = tty
	}

	if m.command.Stdin == nil {
		m.command.Stdin = tty
	}

	m.command.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	if err := m.command.Start(); err != nil {
		_ = pty.Close()
		return 0, fmt.Errorf("could not start command: %w", err)
	}

	m.ptyFile = pty
	return m.command.Process.Pid, nil
}

func (m *PTYMaster) Write(b []byte) (int, error) {
	return m.ptyFile.Write(b)
}

func (m *PTYMaster) Read(b []byte) (int, error) {
	return m.ptyFile.Read(b)
}

func (m *PTYMaster) SetWinSize(rows, cols int) {
	ptyhandler.Setsize(m.ptyFile, &ptyhandler.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (m *PTYMaster) Refresh() {
	// We wanna force the app to re-draw itself, but there doesn't seem to be a way to do that
	// so we fake it by resizing the window quickly, making it smaller and then back big
	m.SetWinSize(Rows-1, Cols)

	go func() {
		time.Sleep(time.Millisecond * 50)
		m.SetWinSize(Rows, Cols)
	}()
}

func (m *PTYMaster) Wait() (err error) {
	err = m.command.Wait()
	return
}

func (m *PTYMaster) Close() (err error) {
	signal.Ignore(syscall.SIGWINCH)

	m.command.Process.Signal(syscall.SIGTERM)
	// TODO: Find a proper wai to close the running command. Perhaps have a timeout after which,
	// if the command hasn't reacted to SIGTERM, then send a SIGKILL
	// (bash for example doesn't finish if only a SIGTERM has been sent)
	m.command.Process.Signal(syscall.SIGKILL)
	return
}
