package server

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type PTYHandler interface {
	Write(data []byte) (int, error)
	Refresh()
}

// TTYServerConfig is used to configure the tty server before it is started
type TTYServerConfig struct {
	Address string
}

// TTYServer represents the instance of a tty server
type TTYServer struct {
	httpServer *http.Server
	config     TTYServerConfig
}

// New creates a new instance
func New(config TTYServerConfig) (server *TTYServer) {
	server = &TTYServer{
		config: config,
	}
	server.httpServer = &http.Server{
		Addr: config.Address,
	}
	routesHandler := mux.NewRouter()

	installHandlers := func(session string) {
		ttyWsPath := "/s/" + session + "/ws"

		routesHandler.HandleFunc(ttyWsPath, func(w http.ResponseWriter, r *http.Request) {
			server.handleTTYWebsocket(w, r)
		})
	}

	installHandlers("local")

	server.httpServer.Handler = routesHandler

	return server
}

func (server *TTYServer) handleTTYWebsocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.WithError(err).Error("cannot create the websocket connection")
		return
	}

	if err := newTerminalSession(conn).Handle(); err != nil {
		return
	}
}

func (server *TTYServer) Run() (err error) {
	err = server.httpServer.ListenAndServe()
	log.Debug("Server finished")
	return
}

func (server *TTYServer) Stop() error {
	log.Debug("Stopping the server")
	return server.httpServer.Close()
}
