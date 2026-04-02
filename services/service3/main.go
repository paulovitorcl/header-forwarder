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

	log.Printf("[service3] GET /whoAreYou — headers: %v", headers)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"service":          "3",
		"message":          "I am 3",
		"received_headers": headers,
	})
}

func main() {
	http.HandleFunc("/whoAreYou", whoAreYou)
	fmt.Println("Service 3 listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
