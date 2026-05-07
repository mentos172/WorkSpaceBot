package main

import (
	"encoding/json"

	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type Task struct {
	UserID    string
	StartedAt time.Time
	StoppedAt *time.Time
	TrackID   string
}

var tasks = make(map[string]*Task)
var mu sync.Mutex
var db *sql.DB
var err error

func main() {
	http.HandleFunc("/start_task", startHandler)
	http.HandleFunc("/stop_task", stopHandler)
	http.HandleFunc("/status", statusHandler)
	db, err = sql.Open("postgres", "host=my_postgres port=5432 user=postgres password=1244 dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal("Ошибка открытия базы данных:", err)
	}
	_, err = db.Exec("SET search_path TO public")
	if err != nil {
		log.Fatal("Ошибка установки search_path:", err)
	}
	// Проверка соединения с базой
	err = db.Ping()
	if err != nil {
		log.Fatal("Не удалось подключиться к базе:", err)
	} else {
		log.Println("Подключение к базе успешно установлено!")
	}

	defer db.Close()
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
	trackID := uuid.New().String()
	tasks[userID] = &Task{UserID: userID, StartedAt: duration, TrackID: trackID}

	resp := map[string]string{
		"message":  "Task started",
		"duration": duration.Format("15:04:05"), // время запуска в формате ЧЧ:ММ:СС
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	// Вставить новую запись в Tracking !!!!! с этим вылет!!!
	var currentDB string
	err = db.QueryRow("SELECT current_database()").Scan(&currentDB)
	if err != nil {
		log.Println("Ошибка получения текущей базы:", err)
	}
	log.Println("Текущая база:", currentDB)
	rows, err := db.Query(`SHOW search_path`)
	if err != nil {
		log.Println("Ошибка при получении search_path:", err)
	} else {
		var path string
		if rows.Next() {
			err = rows.Scan(&path)
			if err != nil {
				log.Println("Ошибка чтения search_path:", err)
			} else {
				log.Println("Текущий search_path:", path)
			}
		}
	}
	_, err = db.Exec(`
    INSERT INTO public.users (user_id)
    VALUES ($1)
	ON CONFLICT (user_id) DO NOTHING
`, userID)
	if err != nil {
		log.Printf("Ошибка при вставке: %v\n", err)
	}

	startTime := duration
	//taskID := uuid.Nil // либо NULL, если task_id пока не известен, замените на нужный

	_, err = db.Exec(`
    INSERT INTO public.tracking (track_id, user_id, start_time)
    VALUES ($1, $2, $3)
`, trackID, userID, startTime)
	if err != nil {
		log.Printf("Ошибка при вставке в tracking: %v\n", err)
	}
	if err != nil {
		log.Println(err)
		http.Error(w, "db error", 500)
		return
	}

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

	_, err = db.Exec(`
    UPDATE public.tracking
    SET stop_time = $1
    WHERE track_id = $2
`, now, task.TrackID)
	if err != nil {
		log.Printf("Ошибка обновления stop_time: %v\n", err)
	}

	_, err = db.Exec(`
    UPDATE public.users
    SET last_tracking = $1
    WHERE user_id = $2
`, now, userID)
	if err != nil {
		log.Printf("Ошибка обновления stop_time: %v\n", err)
	}
}
