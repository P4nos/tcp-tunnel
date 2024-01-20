package main

import (
	"log"
	"os"
	"strconv"
)

func main() {

	localPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalln("Invalid port number")
	}

	c, err := New(localPort)
	if err != nil {
		log.Fatalln("Failed to start client", err)
	}

	defer c.Shutdown()

	for {
		if err := c.HandleIncomingTraffic(); err != nil {
			break
		}
	}
}
