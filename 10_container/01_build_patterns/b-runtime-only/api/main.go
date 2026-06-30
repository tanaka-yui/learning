package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func echo(w http.ResponseWriter, _ *http.Request) {
	host, _ := os.Hostname()
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"lang": "go", "host": host})
}

func main() {
	http.HandleFunc("/api/v1/echo", echo)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
