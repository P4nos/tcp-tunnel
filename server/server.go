package main

import "log"

func main() {
	serverConfig, err := Start()
	if err != nil {

		log.Fatal("failed to start server", err)
	}

	defer serverConfig.Shutdown()

	for {
		serverConfig.HandleConnection()
	}
}

