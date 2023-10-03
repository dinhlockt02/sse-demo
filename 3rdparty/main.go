package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/r3labs/sse/v2"
	"math/rand"
	"net/http"
	"time"
)

const (
	EventChartUpdate    = "chart-update"
	EventNewsUpdate     = "news-update"
	EventProgressUpdate = "progress-update"

	MessagesStreamID = "messages"

	ExecuteComplete = "Execute complete."
)

type News struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type ChartData struct {
	Value int `json:"value"`
	Label int `json:"label"`
}

type ProgressData struct {
	Ongoing bool   `json:"ongoing"`
	Value   int    `json:"value"`
	Message string `json:"message"`
}

var progress = make(chan ProgressData, 100)

var (
	newsChannel = make(chan News, 100)
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.HandleFunc("/chart", ChartHandler())
	r.Get("/news", WatchNewsHandler())
	r.Post("/news", CreateNewsHandler())

	r.Get("/progress", WatchExecuteHandler())
	r.Post("/progress", ExecuteAsync())
	http.ListenAndServe(":3000", r)
}

func ChartHandler() http.HandlerFunc {
	var chartValue = 0
	var timestamp = 0
	func() {
		s := rand.NewSource(time.Now().UnixNano())
		r := rand.New(s)
		go func() {
			for {
				time.Sleep(time.Second * 1)
				var min = 0
				if chartValue-5 >= 0 {
					min = chartValue - 5
				}
				timestamp++
				chartValue = r.Intn(chartValue+5-min) + min
			}
		}()
	}()

	server := sse.New()
	server.CreateStream(MessagesStreamID)
	server.AutoReplay = false
	server.EventTTL = time.Second * 1

	go func() {
		for {
			time.Sleep(time.Second * 1)
			data, err := json.Marshal(&ChartData{
				Value: chartValue,
				Label: timestamp,
			})
			fmt.Println(timestamp)
			if err != nil {
				fmt.Println(err)
				break
			}
			server.Publish(MessagesStreamID, &sse.Event{
				Data:  data,
				Event: []byte(EventChartUpdate),
			})
		}
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		server.ServeHTTP(w, r)
	}
}

func CreateNewsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var news News
		if err := json.NewDecoder(r.Body).Decode(&news); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("fail to read body"))
			return
		}

		newsChannel <- news

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("create news successful"))
		return
	}
}

func WatchNewsHandler() http.HandlerFunc {
	server := sse.New()
	server.AutoReplay = false
	server.CreateStream(MessagesStreamID)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		go func() {
			for news := range newsChannel {
				data, err := json.Marshal(&news)
				if err != nil {
					fmt.Println(err)
					break
				}
				server.Publish(MessagesStreamID, &sse.Event{
					Data:  data,
					Event: []byte(EventNewsUpdate),
				})
			}
		}()

		server.ServeHTTP(w, r)
	}
}

func ExecuteAsync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		go func() {
			for i := 0; i < 100; i++ {
				progress <- ProgressData{
					Ongoing: true,
					Value:   i,
				}
				time.Sleep(300 * time.Millisecond)
			}
			progress <- ProgressData{
				Ongoing: false,
				Value:   0,
				Message: ExecuteComplete,
			}
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("execute success"))
		return
	}
}

func WatchExecuteHandler() http.HandlerFunc {
	server := sse.New()
	server.CreateStream(MessagesStreamID)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		go func() {
			for value := range progress {
				data, err := json.Marshal(&value)
				if err != nil {
					fmt.Println(err)
					break
				}
				server.Publish(MessagesStreamID, &sse.Event{
					Data:  data,
					Event: []byte(EventProgressUpdate),
				})
			}
		}()

		server.ServeHTTP(w, r)
	}
}
