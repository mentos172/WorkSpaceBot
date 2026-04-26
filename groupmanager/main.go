package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type response struct {
	Message string `json:"message"`
}

func main() {
	http.HandleFunc("/add_group", handleAddGroup)
	http.HandleFunc("/add_lead", handleAddLead)

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
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	// и логика добавления лида
	msg := "Лид добавлен для пользователя " + userID

	sendJSON(w, response{Message: msg})
}

func sendJSON(w http.ResponseWriter, resp response) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
