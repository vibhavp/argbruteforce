//+build windows !linux darwin

package main

import (
	"log"
	"os"
)

func startServer() {
	log.Println("Server mode isn't supported on windows/os x")
	os.Exit(1)
}
