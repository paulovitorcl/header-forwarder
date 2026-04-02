package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func whoAreYou(w http.ResponseWriter, r *http.Request) {
	headers := make(map[string]string)
	for key, values := range r.Header {
		headers[key] = values[0]
	}

	log.Printf("[service2] GET /whoAreYou — headers: %v", headers)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"service":          "2",
		"message":          "I am 2",
		"received_headers": headers,
	})
}

func main() {
	http.HandleFunc("/whoAreYou", whoAreYou)
	fmt.Println("Service 2 listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
