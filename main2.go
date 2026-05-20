func handleAddLead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqData struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Lead     string `json:"lead"`
	}

	err := json.NewDecoder(r.Body).Decode(&reqData)
	if err != nil {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	// Обработка данных: например, сохраняем в базу
	// Предположим, есть таблица leads (user_id, username, lead_name)
	_, err = db.Exec("INSERT INTO leads (user_id, username, lead_name) VALUES ($1, $2, $3)", reqData.UserID, reqData.Username, reqData.Lead)
	if err != nil {
		http.Error(w, "Ошибка сохранения данных", http.StatusInternalServerError)
		return
	}

	// Возврат подтверждения
	resp := response{Message: "Лид успешно добавлен"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}