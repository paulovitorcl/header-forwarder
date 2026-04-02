package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

var (
	service2URL = getEnv("SERVICE2_URL", "http://service2")
	service3URL = getEnv("SERVICE3_URL", "http://service3")
)

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func whoAreYou(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service": "1",
		"message": "I am 1",
	})
}

// callService makes a plain HTTP GET with no additional headers.
// The HTTP_PROXY env var causes Go's default client to route this
// through the header-forwarder sidecar, which injects x-journey-id.
func callService(target string) map[string]interface{} {
	resp, err := http.Get(target + "/whoAreYou")
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return map[string]interface{}{"raw": string(body)}
	}
	return result
}

func startTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[service1] /startTest — calling service2 and service3 (no headers added by this app)")

	result2 := callService(service2URL)
	result3 := callService(service3URL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"note":              "Header propagation is done by the sidecar proxy, NOT by this application",
		"service2_response": result2,
		"service3_response": result3,
	})
}

func main() {
	http.HandleFunc("/whoAreYou", whoAreYou)
	http.HandleFunc("/startTest", startTest)
	fmt.Println("Service 1 listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
