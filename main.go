package main

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message: "Hello, World!",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func basicHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message: "Who are you?",
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(response.Message))
}

func main() {
	http.HandleFunc("/hello", helloHandler)
	http.HandleFunc("/basic", basicHandler)
}
