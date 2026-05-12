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
	TaskName  string
	StartedAt time.Time
	StoppedAt *time.Time
	TrackID   string
	TaskID    string
}

var tasks = make(map[string]*Task)
var mu sync.Mutex
var db *sql.DB
var err error

func main() {
	http.HandleFunc("/stop_task", stopHandler)
	http.HandleFunc("/start_task", taskHandler)
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

func stopHandler(w http.ResponseWriter, r *http.Request) {
	var userID, comment string

	switch r.Method {
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		userID = r.Form.Get("user_id")
		comment = r.Form.Get("comment")
	default:
		userID = r.URL.Query().Get("user_id")
		comment = r.URL.Query().Get("comment")
	}

	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	task, exists := tasks[userID]
	if !exists {
		http.Error(w, "Нет запущенной задачи", http.StatusNotFound)
		return
	}
	now := time.Now()
	task.StoppedAt = &now

	duration := task.StoppedAt.Sub(task.StartedAt).Round(time.Second)

	delete(tasks, userID) // Очистим задачу

	resp := map[string]string{
		"message":  "Задача выполнена",
		"duration": duration.String(),
		"comment":  comment,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	_, err = db.Exec(`
        UPDATE public.tasks
        SET t_description = $1 
        WHERE task_id = $2
    `, comment, task.TaskID)
	if err != nil {
		log.Printf("Ошибка обновления stop_time/comment: %v\n", err)
	}

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

func taskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST supported", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	userID := r.Form.Get("user_id")
	username := r.Form.Get("username")
	taskName := r.Form.Get("task")
	//username := r.Form.Get("username")
	if userID == "" || taskName == "" {
		http.Error(w, "user_id and task required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Уже есть активная задача
	if _, exists := tasks[userID]; exists {
		http.Error(w, "Задача уже запущена", http.StatusConflict)
		return
	}

	now := time.Now()
	trackID := uuid.New().String()
	taskID := uuid.New().String()
	tasks[userID] = &Task{
		UserID:    userID,
		TaskName:  taskName,
		StartedAt: now,
		TrackID:   trackID,
		TaskID:    taskID,
	}
	resp := map[string]string{
		"message":  "Задача начата",
		"task":     taskName,
		"duration": now.Format("15:04:05"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// Добавляем или обновляем пользователя с именем
	_, err = db.Exec(`
		INSERT INTO public.users (user_id, username)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET username = EXCLUDED.username
	`, userID, username)
	if err != nil {
		log.Printf("Ошибка при вставке/обновлении пользователя: %v\n", err)
	}

	_, err = db.Exec(`
    INSERT INTO public.tasks(task_id, t_name)
    VALUES ($1, $2)
`, taskID, taskName)
	if err != nil {
		log.Printf("Ошибка при вставке: %v\n", err)
	}

	startTime := now
	//taskID := uuid.Nil // либо NULL, если task_id пока не известен, замените на нужный

	_, err = db.Exec(`
    INSERT INTO public.tracking (track_id, user_id, start_time, task_id)
    VALUES ($1, $2, $3, $4)
`, trackID, userID, startTime, taskID)
	if err != nil {
		log.Printf("Ошибка при вставке в tracking: %v\n", err)
	}
	if err != nil {
		log.Println(err)
		http.Error(w, "db error", 500)
		return
	}

}
