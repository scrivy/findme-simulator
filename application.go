package main

import (
	"io"
	"log"
	"time"

	"golang.org/x/net/websocket"
)

type message struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

type location struct {
	Id       string    `json:"id"`
	Latlng   []float64 `json:"latlng"`
	Accuracy float64   `json:"accuracy"`
}

func main() {
	n := 50
	for i := 0; i < n; i++ {
		time.Sleep(time.Duration(1000/n) * time.Millisecond)
		go connectAndSendUpdates()
	}

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Println("say something about performance")
		}
	}
}

func connectAndSendUpdates() {
	ws, err := websocket.Dial("ws://localhost:5000/ws", "", "http://localhost/")
	if err != nil {
		log.Println(err)
		return
	}

	ticker := time.NewTicker(time.Second)

	var m message
	for {
		err = websocket.JSON.Receive(ws, &m)
		if err != nil {
			switch err {
			case io.EOF:

			default:
				log.Println("err = websocket.JSON.Receive(ws, &m):", err)
			}
			break
		}

		// log.Println("Received message:", m.Action)
		// log.Printf("%+v\n", m.Data)

		select {
		case <-ticker.C:
			websocket.JSON.Send(ws, message{
				Action: "updateLocation",
				Data: map[string]interface{}{
					"latlng": []float64{
						37.7812681,
						-122.4075934,
					},
					"accuracy": 31,
				},
			})
		}
	}

}
