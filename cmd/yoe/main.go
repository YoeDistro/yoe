package main

import (
	"log"
	"os"
)

func main() {
	osBase := os.Getenv("OE_BASE")
	if osBase == "" {
		log.Fatalln("OE_BASE must be set")
	}
}
