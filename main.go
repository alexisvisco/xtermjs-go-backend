package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"test-xterm-server/ptymaster"
	"test-xterm-server/server"
)

func main() {
	ptyMaster := ptymaster.New()
	err := ptyMaster.Start("/bin/zsh", nil, os.Environ())
	if err != nil {
		panic(fmt.Errorf("Failed to start pty master: %s", err.Error()))
		return
	}

	ptyMaster.MakeRaw()
	defer ptyMaster.Close()
	config := server.TTYServerConfig{
		Address: ":3000",
		PTY:     ptyMaster,
	}

	server := server.New(config)

	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		server.WindowSize(cols, rows)
	}

	go func() {
		log.WithField("address", config.Address).Info("Starting server")
		err := server.Run()
		if err != nil {
			ptyMaster.Close()
			log.Errorf("Server finished: %s", err.Error())
		}
	}()

	go func() {
		_, err := io.Copy(server, ptyMaster)
		if err != nil {
			ptyMaster.Close()
		}
	}()

	ptyMaster.Wait()
	fmt.Printf("tty-share finished\n\n\r")
	server.Stop()
}
