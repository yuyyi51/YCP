package main

import (
	"log"
	"os"
)

func main() {
	err := createApp().Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
