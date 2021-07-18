package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"github.com/leighmcculloch/ortc/ortc"
)

func main() {
	log.Println("Starting...")
	ortc := ortc.NewORTC()
	localToken, err := ortc.LocalToken()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Local Token:")
	fmt.Println(localToken)

	fmt.Println("Remote Token:")
	reader := bufio.NewReader(os.Stdin)
	remoteToken, _ := reader.ReadString('\n')
	err = ortc.Start(remoteToken)
	if err != nil {
		log.Fatal(err)
	}

	ortc.OnMessage(func(msg []byte) {
		fmt.Println("Received message:", string(msg))
	})

	for {
		fmt.Print("> ")
		msg, _ := reader.ReadBytes('\n')
		err = ortc.SendMessage(msg)
		if err != nil {
			log.Fatal(err)
		}
	}
}
