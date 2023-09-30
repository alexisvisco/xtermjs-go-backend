package server

import (
	"fmt"
	"io"
	"os"
	"test-xterm-server/ptymaster"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type terminalSession struct {
	wsConnectionEndEvent chan bool
	processEndEvent      chan bool

	pty          *ptymaster.PTYMaster
	wsConnection *websocket.Conn
	wsMessenger  *WSMessager

	logger *logrus.Entry
}

func newTerminalSession(wsConn *websocket.Conn) *terminalSession {
	ts := &terminalSession{
		wsConnectionEndEvent: make(chan bool),
		processEndEvent:      make(chan bool),
		pty:                  ptymaster.New(),
		wsConnection:         wsConn,
		logger:               logrus.WithField("component", "terminal-session"),
	}

	wsConn.SetCloseHandler(func(code int, text string) error {
		ts.wsConnectionEndEvent <- true
		close(ts.wsConnectionEndEvent)
		return nil
	})

	return ts
}

func (t *terminalSession) Handle() error {
	pid, err := t.pty.Start("/bin/zsh", nil, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to start pty master: %s", err.Error())
	}

	t.logger = t.logger.WithField("pid", pid)

	t.wsMessenger = NewWSMessenger(t.wsConnection, t.logger)

	t.logger.Info("creating connection with a terminal pty master")

	defer t.safeClosePtyMaster()
	defer t.safeCloseWSConnection()

	// copy every bytes from the pty to the websocket connection
	go t.copyPTYToWS()

	go t.waitPtyMaster()

	t.listen()

	return nil
}

// listen for events from the websocket connection
// it will write on the pty master the data received from the websocket connection (xtermjs)
func (t *terminalSession) listen() {
	for {
		select {
		case <-t.processEndEvent:
			t.logger.Info("ws end")
			return
		case <-t.wsConnectionEndEvent:
			t.logger.Info("process end")
			return
		case data := <-t.wsMessenger.Next():
			_, _ = t.pty.Write(data)
			break
		}
	}
}

func (t *terminalSession) safeClosePtyMaster() {
	if err := t.pty.Close(); err != nil {
		t.logger.WithError(err).Error("could not close pty master")
	}
}

func (t *terminalSession) safeCloseWSConnection() {
	if err := t.wsConnection.Close(); err != nil {
		t.logger.WithError(err).Error("could not close receiver connection")
	}
}

func (t *terminalSession) copyPTYToWS() {
	_, err := io.Copy(t.wsMessenger, t.pty)
	if err != nil {
		t.logger.Warnf("unable to copy first bytes from pty master: %s", err.Error())
	}
}

func (t *terminalSession) waitPtyMaster() {
	if err := t.pty.Wait(); err != nil {
		t.logger.WithError(err).Error("finished waiting for pty master")
	}

	t.processEndEvent <- true

	t.logger.Info("process end")
}
