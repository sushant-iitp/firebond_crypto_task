package main

import (
	"net/http"
)

func BasicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Who are you?"))
}
