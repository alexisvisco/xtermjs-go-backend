package server

import (
	"container/list"
	"sync"
	"test-xterm-server/ptymaster"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type ttyShareSession struct {
	mainRWLock sync.RWMutex

	ttyProtoConnections *list.List
	isAlive             bool
	lastWindowSizeMsg   MsgTTYWinSize
	ptyHandler          ptymaster.PTYHandler
}

func copyList(l *list.List) *list.List {
	newList := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		newList.PushBack(e.Value)
	}
	return newList
}

func newTTYShareSession(ptyHandler ptymaster.PTYHandler) *ttyShareSession {
	ttyShareSession := &ttyShareSession{
		ttyProtoConnections: list.New(),
		ptyHandler:          ptyHandler,
	}

	return ttyShareSession
}

func (session *ttyShareSession) WindowSize(cols, rows int) error {
	session.mainRWLock.Lock()
	session.lastWindowSizeMsg = MsgTTYWinSize{Cols: cols, Rows: rows}
	session.mainRWLock.Unlock()

	session.forEachReceiverLock(func(rcvConn *TTYProtocolWSLocked) bool {
		rcvConn.SetWinSize(cols, rows)
		return true
	})
	return nil
}

func (session *ttyShareSession) Write(data []byte) (int, error) {
	session.forEachReceiverLock(func(rcvConn *TTYProtocolWSLocked) bool {
		//fmt.Println("Write data: ", string(data))
		rcvConn.Write(data)
		return true
	})
	return len(data), nil
}

// Runs the callback cb for each of the receivers in the list of the receivers, as it was when
// this function was called. Note that there might be receivers which might have lost
// the connection since this function was called.
// Return false in the callback to not continue for the rest of the receivers
func (session *ttyShareSession) forEachReceiverLock(cb func(rcvConn *TTYProtocolWSLocked) bool) {
	session.mainRWLock.RLock()
	// TODO: Maybe find a better way?
	rcvsCopy := copyList(session.ttyProtoConnections)
	session.mainRWLock.RUnlock()

	for receiverE := rcvsCopy.Front(); receiverE != nil; receiverE = receiverE.Next() {
		receiver := receiverE.Value.(*TTYProtocolWSLocked)
		if !cb(receiver) {
			break
		}
	}
}

// HandleWSConnection handles a new WS connection from a receiver. It will add the connection to
// the list of the receivers of this session, and will wait until the connection is closed.
// When the connection is closed, it will remove the connection from the list of the receivers.
func (session *ttyShareSession) HandleWSConnection(wsConn *websocket.Conn) {

	protoConn := NewTTYProtocolWSLocked(wsConn)

	session.mainRWLock.Lock()
	rcvHandleEl := session.ttyProtoConnections.PushBack(protoConn)
	winSize := session.lastWindowSizeMsg
	session.mainRWLock.Unlock()

	log.Debugf("New WS connection (%s). Serving ..", wsConn.RemoteAddr().String())

	// Sending the initial size of the window, if we have one
	protoConn.SetWinSize(winSize.Cols, winSize.Rows)

	// Wait until the TTYReceiver will close the connection on its end
	for {
		err := protoConn.ReadAndHandle(
			func(data []byte) {
				session.ptyHandler.Write(data)
			},
			func(cols, rows int) {
				session.ptyHandler.Refresh()
			},
		)

		if err != nil {
			log.Debugf("Finished the WS reading loop: %s", err.Error())
			break
		}
	}

	// Remove the recevier from the list of the receiver of this session, so we need to write-lock
	session.mainRWLock.Lock()
	session.ttyProtoConnections.Remove(rcvHandleEl)
	session.mainRWLock.Unlock()

	wsConn.Close()
	log.Debugf("Closed receiver connection")
}
