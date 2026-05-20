package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	_ "github.com/lib/pq"
)

type response struct {
	Message string `json:"message"`
}

var mu sync.Mutex
var db *sql.DB
var err error

func main() {
	http.HandleFunc("/add_group", handleAddGroup)
	http.HandleFunc("/add_lead", handleAddLead)
	http.HandleFunc("/get_user_status", handleGetUserStatus)
	//http.HandleFunc("/delete_group", handleDeleteGroup)
	//http.HandleFunc("/add_user", handleAddUser)

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

	log.Println("Бизнес-сервис запущен на :8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal("Ошибка запуска сервера:", err)
	}

}

func handleAddGroup(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	// здесь могла бы быть ваша логика добавления группы
	msg := "Группа добавлена для пользователя " + userID

	sendJSON(w, response{Message: msg})
}

func handleAddLead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"message":"Only POST supported"}`, http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"message":"invalid form"}`, http.StatusBadRequest)
		return
	}
	userID := r.Form.Get("user_id")
	leadID := r.Form.Get("leadText") // leadID - id пользователя, которого делаем лидом

	if userID == "" || leadID == "" {
		http.Error(w, `{"message":"не указан пользователь (отправитель или назначаемый)"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Проверка, является ли userID админом (god)
	var isGod bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND status = 'god')", userID).Scan(&isGod)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки пользователя"}`, http.StatusInternalServerError)
		return
	}
	if !isGod {
		json.NewEncoder(w).Encode(response{Message: "У вас нет прав на данное действие."})
		return
	}

	// Проверка, что назначаемый пользователь существует
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)", leadID).Scan(&exists)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки пользователя"}`, http.StatusInternalServerError)
		return
	}
	if !exists {
		json.NewEncoder(w).Encode(response{Message: "Пользователя с таким ID нет в базе"})
		return
	}

	// Назначаем пользователю статус "lead"
	_, err = db.Exec("UPDATE users SET status = 'lead' WHERE user_id = $1;", leadID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка сохранения данных"}`, http.StatusInternalServerError)
		return
	}

	// Успешный ответ, можно добавить id лида
	json.NewEncoder(w).Encode(response{Message: "Лид успешно добавлен"})
}

func handleGetUserStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, `{"message":"user_id required"}`, http.StatusBadRequest)
		return
	}

	// Чтение статуса из БД
	var status string
	err := db.QueryRow("SELECT status FROM users WHERE user_id = $1", userID).Scan(&status)
	if err == sql.ErrNoRows {
		json.NewEncoder(w).Encode(response{Message: "Пользователь не найден"})
		return
	} else if err != nil {
		http.Error(w, `{"message":"Ошибка чтения из БД"}`, http.StatusInternalServerError)
		return
	}

	// Возвращаем статус в json
	resp := map[string]string{
		"status":  status,
		"message": "OK",
	}
	json.NewEncoder(w).Encode(resp)
}

func sendJSON(w http.ResponseWriter, resp response) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
