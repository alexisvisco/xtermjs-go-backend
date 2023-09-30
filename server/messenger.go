package server

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"io"
)

const (
	MsgIDWrite = "Write"
)

type MsgWrapper struct {
	Type string
	Data []byte
}

type MsgTTYWrite struct {
	Data []byte
	Size int
}

type WSMessager struct {
	ws     *websocket.Conn
	logger *log.Entry
}

func NewWSMessenger(ws *websocket.Conn, l *log.Entry) *WSMessager {
	return &WSMessager{
		ws:     ws,
		logger: l.WithField("component", "ws-messager"),
	}
}

func marshalMsg(aMessage interface{}) (_ []byte, err error) {
	var msg MsgWrapper

	if writeMsg, ok := aMessage.(MsgTTYWrite); ok {
		msg.Type = MsgIDWrite
		msg.Data, err = json.Marshal(writeMsg)
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

	return nil, nil
}

// Next reads a message from the websocket connection
func (m *WSMessager) Next() chan []byte {
	writerChan := make(chan []byte, 1)

	var msg MsgWrapper

	go func() {
		_, r, err := tryReadNext(m)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				m.logger.WithError(err).Error("unexpected close error")
			}

			close(writerChan)
			return
		}

		err = json.NewDecoder(r).Decode(&msg)

		if err != nil {
			return
		}

		switch msg.Type {
		case MsgIDWrite:
			var msgWrite MsgTTYWrite
			err = json.Unmarshal(msg.Data, &msgWrite)
			if err == nil {
				writerChan <- msgWrite.Data
				close(writerChan)
			}
		}
	}()

	return writerChan
}

// Function to send data from one the sender to the server and the other way around.
func (m *WSMessager) Write(buff []byte) (n int, err error) {
	msgWrite := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}
	data, err := marshalMsg(msgWrite)
	if err != nil {
		return 0, err
	}

	n, err = len(buff), m.ws.WriteMessage(websocket.TextMessage, data)
	return
}

func tryReadNext(handler *WSMessager) (mt int, reader io.Reader, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = io.EOF
		}
	}()
	return handler.ws.NextReader()
}
