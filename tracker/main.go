package main

import (
	"encoding/json"

	"log"
	"net/http"
	"sync"
	"time"
)

type Task struct {
	UserID    string
	StartedAt time.Time
	StoppedAt *time.Time
}

var tasks = make(map[string]*Task)
var mu sync.Mutex

func main() {
	http.HandleFunc("/start", startHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/status", statusHandler)
	log.Println("Tracker service running at :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))

}

func startHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	if _, exists := tasks[userID]; exists {
		http.Error(w, "task already started", http.StatusConflict)
		return
	}

	duration := time.Now()
	tasks[userID] = &Task{UserID: userID, StartedAt: duration}

	resp := map[string]string{
		"message":  "Task started",
		"duration": duration.Format("15:04:05"), // время запуска в формате ЧЧ:ММ:СС
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	task, exists := tasks[userID]
	if !exists {
		http.Error(w, "no task started", http.StatusNotFound)
		return
	}

	// Вычислить длительность с момента старта до сейчас
	duration := time.Since(task.StartedAt).Round(time.Second)

	resp := map[string]string{
		"message":  "Task is running",
		"duration": duration.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	task, exists := tasks[userID]
	if !exists {
		http.Error(w, "no task started", http.StatusNotFound)
		return
	}
	now := time.Now()
	task.StoppedAt = &now

	duration := task.StoppedAt.Sub(task.StartedAt).Round(time.Second)

	delete(tasks, userID) // Очистим задачу

	resp := map[string]string{"message": "Task stopped", "duration": duration.String()}
	json.NewEncoder(w).Encode(resp)
}
