// Example: building a small HTTP service on top of the VeloxQuant Go SDK,
// exposing memory estimation and chat as JSON endpoints.
package main

import (
	"encoding/json"
	"log"
	"net/http"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func main() {
	client, err := veloxquant.NewClient(veloxquant.WithAutoDetect())
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/system", func(w http.ResponseWriter, r *http.Request) {
		info, err := client.System.Info(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(info)
	})

	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		var req veloxquant.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp, err := client.Chat(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
