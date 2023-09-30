package server

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// Config is used to configure the tty server before it is started
type Config struct {
	Address string
}

// Server represents the instance of a tty server
type Server struct {
	httpServer *http.Server
	config     Config
}

// New creates a new instance
func New(config Config) (server *Server) {
	server = &Server{
		config: config,
	}
	server.httpServer = &http.Server{
		Addr: config.Address,
	}
	routesHandler := mux.NewRouter()

	routesHandler.HandleFunc("/s/local/ws", func(w http.ResponseWriter, r *http.Request) {
		server.handleTerminalWebsocket(w, r)
	})

	server.httpServer.Handler = routesHandler

	return server
}

func (server *Server) handleTerminalWebsocket(w http.ResponseWriter, r *http.Request) {
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

func (server *Server) Run() (err error) {
	err = server.httpServer.ListenAndServe()
	log.Debug("Server finished")
	return
}

func (server *Server) Stop() error {
	log.Debug("Stopping the server")
	return server.httpServer.Close()
}
