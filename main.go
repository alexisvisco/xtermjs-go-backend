package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"test-xterm-server/server"
)

func main() {
	config := server.Config{
		Address: ":3000",
	}

	logrus.Info("Starting server on ", config.Address)
	if err := server.New(config).Run(); err != nil {
		panic(fmt.Errorf("cannot start the server: %s", err.Error()))
	}
}
