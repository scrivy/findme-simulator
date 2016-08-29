package main

import (
	"encoding/json"
	"fmt"
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
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
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

var jsonBytes []byte

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

	var err error
	jsonBytes, err = json.Marshal(message{
		Action: "updateLocation",
		Data: map[string]interface{}{
			"latlng": []float64{
				37.7812681,
				-122.4075934,
			},
			"accuracy": 31,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	var txCount, rxCount, latency uint64
	for i := 0; i < n; i++ {
		time.Sleep(time.Duration(1000/n) * time.Millisecond)
		go connectAndSendUpdates(&txCount, &rxCount, &latency)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:5050", nil))
	}()


	counters, err := net.IOCounters(true)
	if err != nil {
		log.Fatal(err)
	}
	lo := counters[0]

	var interval, seconds, prevBytesSent, prevBytesRecv uint64
	interval = 5
	seconds = interval
	prevBytesSent = lo.BytesSent
	prevBytesRecv = lo.BytesRecv
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	var tmpRxCount, thisLatency uint64
	for {
		select {
		case <-ticker.C:
			tmpRxCount = atomic.LoadUint64(&rxCount)
			thisLatency = uint64(atomic.LoadUint64(&latency)) / tmpRxCount / 1000000

			fmt.Printf(
				"\n%6d tx msg / sec\n%6d rx msg / sec\n%6d ms latency\n",
				atomic.LoadUint64(&txCount)/seconds,
				tmpRxCount/seconds,
				thisLatency)
			if thisLatency > 1000 {
				log.Panicln("Exceded 1s latency")
			}
			seconds += interval

			timesArr, err := cpu.Percent(0, false)
			if err != nil {
				log.Fatal(err)
			}
			times := timesArr[0]
			fmt.Printf("%6.1f cpu used\n", times)

			virtMem, err := mem.VirtualMemory()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%6d mB used mem\n", virtMem.Used / 1000000)

			counters, err = net.IOCounters(true)
			if err != nil {
				log.Fatal(err)
			}
			lo = counters[0]
			fmt.Printf("%6d kB tx / sec \n", (lo.BytesSent - prevBytesSent)/5000)
			fmt.Printf("%6d kB rx / sec \n", (lo.BytesRecv - prevBytesRecv)/5000)

			prevBytesSent = lo.BytesSent
			prevBytesRecv = lo.BytesRecv
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

	go func() {
		var rxm message
		var duration time.Duration
		var rxJsonBytes []byte
	Read:
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
					break Read
				}

				json.Unmarshal(rxJsonBytes, &rxm)

				duration = time.Since(rxm.Date)
				atomic.AddUint64(latency, uint64(duration.Nanoseconds()))
				atomic.AddUint64(rxCountP, 1)
			}
		}
	}()

Write:
	for {
		select {
		case <-ticker.C:
			err = conn.WriteMessage(websocket.TextMessage, jsonBytes)
			if err != nil {
				logErr(err)
				break Write
			}
			atomic.AddUint64(txCountP, 1)
		}
	}

}

func logErr(err error) {
	log.Println(err)
	debug.PrintStack()
}
