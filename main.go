package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	EventChartUpdate = "chart-update"
	EventNewsUpdate  = "news-update"
)

type News struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

var (
	newsChannel = make(chan News, 100)
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.HandleFunc("/chart", ChartHandler())
	r.Get("/news", WatchNewsHandler())
	r.Post("/news", CreateNewsHandler())
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
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")

		for {
			time.Sleep(time.Second * 1)
			event, err := formatServerSentEvent(EventChartUpdate, struct {
				Value int `json:"value"`
				Label int `json:"label"`
			}{
				Label: timestamp,
				Value: chartValue,
			})
			if err != nil {
				fmt.Println(err)
				break
			}

			_, err = fmt.Fprint(w, event)
			if err != nil {
				fmt.Println(err)
				break
			}

			flusher.Flush()
		}
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
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()
		for news := range newsChannel {
			fmt.Println("new news")
			event, err := formatServerSentEvent(EventNewsUpdate, news)
			if err != nil {
				fmt.Println(err)
				break
			}

			_, err = fmt.Fprint(w, event)
			if err != nil {
				fmt.Println(err)
				break
			}
			flusher.Flush()
		}
	}
}

func formatServerSentEvent(event string, data any) (string, error) {
	m := map[string]any{
		"data": data,
	}

	buff := bytes.NewBuffer([]byte{})

	encoder := json.NewEncoder(buff)

	err := encoder.Encode(m)
	if err != nil {
		return "", err
	}

	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("event: %s\n", event))
	sb.WriteString(fmt.Sprintf("data: %v\n", buff.String()))

	return sb.String(), nil
}
