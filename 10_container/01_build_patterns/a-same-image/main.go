package main

import (
	"fmt"
	"net/http"
	"os"
)

func healthz(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") }

func envHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "APP_ENV=%s\n", os.Getenv("APP_ENV"))
}

func main() {
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/env", envHandler)
	addr := ":" + getEnv("PORT", "8080")
	fmt.Println("listening", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
