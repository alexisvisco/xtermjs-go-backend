package server

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
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
	PTY     PTYHandler
}

// TTYServer represents the instance of a tty server
type TTYServer struct {
	httpServer       *http.Server
	config           TTYServerConfig
	session          *ttyShareSession
	muxTunnelSession *yamux.Session
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

	// Install the same routes on both the /local/ and /<SessionID>/. The session ID is received
	// from the tty-proxy server, if a public session is involved.
	installHandlers("local")

	server.httpServer.Handler = routesHandler
	server.session = newTTYShareSession(config.PTY)

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
		log.Error("Cannot create the WS connection: ", err.Error())
		return
	}

	// On a new connection, ask for a refresh/redraw of the terminal app
	server.config.PTY.Refresh()
	server.session.HandleWSConnection(conn)
}

func (server *TTYServer) Run() (err error) {
	err = server.httpServer.ListenAndServe()
	log.Debug("Server finished")
	return
}

func (server *TTYServer) Write(buff []byte) (written int, err error) {
	return server.session.Write(buff)
}

func (server *TTYServer) WindowSize(cols, rows int) (err error) {
	return server.session.WindowSize(cols, rows)
}

func (server *TTYServer) Stop() error {
	log.Debug("Stopping the server")
	if server.muxTunnelSession != nil {
		server.muxTunnelSession.Close()
	}
	return server.httpServer.Close()
}
