package ptymaster

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type onWindowChangedCB func(int, int)

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

func isStdinTerminal() bool {
	return terminal.IsTerminal(0)
}

func (m *PTYMaster) Start(command string, args []string, envVars []string) (err error) {
	m.command = exec.Command(command, args...)
	m.command.Env = envVars
	m.ptyFile, err = pty.Start(m.command)

	if err != nil {
		return
	}

	// Set the initial window size
	cols, rows := 140, 32

	m.SetWinSize(rows, cols)
	return
}

func (m *PTYMaster) MakeRaw() (err error) {
	// Save the initial state of the terminal, before making it RAW. Note that this terminal is the
	// terminal under which the tty-share command has been started, and it's identified via the
	// stdin file descriptor (0 in this case)
	// We need to make this terminal RAW so that when the command (passed here as a string, a shell
	// usually), is receiving all the input, including the special characters:
	// so no SIGINT for Ctrl-C, but the RAW character data, so no line discipline.
	// Read more here: https://www.linusakesson.net/programming/tty/
	m.terminalInitState, err = terminal.MakeRaw(int(os.Stdin.Fd()))
	return
}

func (m *PTYMaster) GetWinSize() (int, int, error) {
	return 140, 32, nil
}

func (m *PTYMaster) Write(b []byte) (int, error) {
	return m.ptyFile.Write(b)
}

func (m *PTYMaster) Read(b []byte) (int, error) {
	return m.ptyFile.Read(b)
}

func (m *PTYMaster) SetWinSize(rows, cols int) {
	winSize := pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}
	pty.Setsize(m.ptyFile, &winSize)
}

func (m *PTYMaster) Refresh() {
	// We wanna force the app to re-draw itself, but there doesn't seem to be a way to do that
	// so we fake it by resizing the window quickly, making it smaller and then back big
	cols, rows, err := m.GetWinSize()

	if err != nil {
		return
	}

	m.SetWinSize(rows-1, cols)

	go func() {
		time.Sleep(time.Millisecond * 50)
		m.SetWinSize(rows, cols)
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

func onWindowChanges(wcCB onWindowChangedCB) {
	wcChan := make(chan os.Signal, 1)
	signal.Notify(wcChan, syscall.SIGWINCH)
	// The interface for getting window changes from the pty slave to its process, is via signals.
	// In our case here, the tty-share command (built in this project) is the client, which should
	// get notified if the terminal window in which it runs has changed. To get that, it needs to
	// register for SIGWINCH signal, which is used by the kernel to tell process that the window
	// has changed its dimentions.
	// Read more here: https://www.linusakesson.net/programming/tty/
	// Shortly, ioctl calls are used to communicate from the process to the pty slave device,
	// and signals are used for the communiation in the reverse direction: from the pty slave
	// device to the process.

	for {
		select {
		case <-wcChan:
			cols, rows, err := terminal.GetSize(0)
			if err == nil {
				wcCB(cols, rows)
			} else {
				log.Warnf("Can't get window size: %s", err.Error())
			}
		}
	}
}
