package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strconv"
	"sync/atomic"
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
	var txCount, rxCount, txTotal, rxTotal uint64

	var n int
	if len(os.Args) == 2 {
		num, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatal("couldn't parse 2nd arg: ", err)
		}
		n = num
	} else {
		n = 100
	}

	for i := 0; i < n; i++ {
		time.Sleep(time.Duration(1000/n) * time.Millisecond)
		go connectAndSendUpdates(&txCount, &rxCount)
	}

	var interval uint64
	interval = 5
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		select {
		case <-ticker.C:
			txNewTotal := atomic.LoadUint64(&txCount)
			rxNewTotal := atomic.LoadUint64(&rxCount)
			log.Printf(
				"\n%6d tx msg / sec\n%6d rx msg / sec",
				(txNewTotal-txTotal)/interval,
				(rxNewTotal-rxTotal)/interval)
			txTotal = txNewTotal
			rxTotal = rxNewTotal
		}
	}
}

func connectAndSendUpdates(txCountP *uint64, rxCountP *uint64) {
	ws, err := websocket.Dial("ws://localhost:5000/ws", "", "http://localhost/")
	if err != nil {
		log.Println(err)
		return
	}

	ticker := time.NewTicker(time.Second)

	m := message{
		Action: "updateLocation",
		Data: map[string]interface{}{
			"latlng": []float64{
				37.7812681,
				-122.4075934,
			},
			"accuracy": 31,
		},
	}
	jsonBytes, err := json.Marshal(&m)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		var rxm message
		for {
			err = websocket.JSON.Receive(ws, &rxm)
			if err != nil {
				switch err {
				case io.EOF:

				default:
					log.Println("err = websocket.JSON.Receive(ws, &m):", err)
				}
				break
			}

			atomic.AddUint64(rxCountP, 1)
		}
	}()

	for {
		select {
		case <-ticker.C:
			_, err := ws.Write(jsonBytes)
			if err != nil {
				log.Println(err)
			}
			atomic.AddUint64(txCountP, 1)
		}
	}

}
