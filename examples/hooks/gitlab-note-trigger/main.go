package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sanix-darker/prev/internal/hooks/gitlabnote"
)

func main() {
	addr := envOrDefault("PREV_GITLAB_NOTE_HOOK_ADDR", ":8091")
	secret := os.Getenv("PREV_GITLAB_NOTE_HOOK_SECRET")
	http.HandleFunc("/gitlab/note", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := readBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		trigger, ok, err := gitlabnote.ParseTriggerRequest(secret, r.Header, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		args := gitlabnote.BuildPrevReviewArgs(trigger)
		cmd := exec.CommandContext(ctx, "prev", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			http.Error(w, fmt.Sprintf("prev review failed: %v", err), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
