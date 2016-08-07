package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type message struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
	Date   time.Time
}

type location struct {
	Id       string    `json:"id"`
	Latlng   []float64 `json:"latlng"`
	Accuracy float64   `json:"accuracy"`
}

func main() {
	var n int // number of concurent connections
	if len(os.Args) == 2 {
		num, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatal("couldn't parse 2nd arg: ", err)
		}
		n = num
	} else {
		n = 100
	}

	var txCount, rxCount, latency uint64
	for i := 0; i < n; i++ {
		time.Sleep(time.Duration(1000/n) * time.Millisecond)
		go connectAndSendUpdates(&txCount, &rxCount, &latency)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:5050", nil))
	}()

	var interval, seconds uint64
	interval = 5
	seconds = interval
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	var tmpRxCount, thisLatency uint64
	for {
		select {
		case <-ticker.C:
			tmpRxCount = atomic.LoadUint64(&rxCount)
			thisLatency = uint64(atomic.LoadUint64(&latency)) / tmpRxCount

			log.Printf(
				"\n%6d tx msg / sec\n%6d rx msg / sec\n%6d ns latency",
				atomic.LoadUint64(&txCount)/seconds,
				tmpRxCount/seconds,
				thisLatency)
			if thisLatency > 1000000000 {
				log.Panicln("Exceded 1s latency")
			}
			seconds += interval
		}
	}
}

func connectAndSendUpdates(txCountP *uint64, rxCountP *uint64, latency *uint64) {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws://localhost:5000/ws", nil)
	if err != nil {
		logErr(err)
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
		var duration time.Duration
		var rxJsonBytes []byte
		for {
			messageType, r, err := conn.NextReader()
			if err != nil {
				switch err {
				case io.EOF:
				default:
					logErr(err)
				}
				break
			}

			switch messageType {
			case websocket.TextMessage:
				rxJsonBytes, err = ioutil.ReadAll(r)
				if err != nil {
					logErr(err)
					continue
				}

				json.Unmarshal(rxJsonBytes, &rxm)

				duration = time.Since(rxm.Date)
				atomic.AddUint64(latency, uint64(duration.Nanoseconds()))
				atomic.AddUint64(rxCountP, 1)
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			err = conn.WriteMessage(websocket.TextMessage, jsonBytes)
			if err != nil {
				logErr(err)
			}
			atomic.AddUint64(txCountP, 1)
		}
	}

}

func logErr(err error) {
	log.Println(err)
	debug.PrintStack()
}
