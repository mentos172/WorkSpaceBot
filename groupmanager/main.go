package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
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
	http.HandleFunc("/delete_group", handleDeleteGroup)
	http.HandleFunc("/add_user", handleAddUser)
	http.HandleFunc("/delete_user", handleDeleteUser)

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

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
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
	input := r.Form.Get("leadText")

	if userID == "" || input == "" {
		http.Error(w, `{"message":"не указан пользователь (отправитель или назначаемый)"}`, http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(input, " ", 2)
	var wuserID, groupID string
	if len(parts) >= 2 {
		wuserID = parts[0]
		groupID = parts[1]
	} else {
		// Если разделитель не найден, присваиваем весь текст первой переменной
		wuserID = input
		groupID = ""
	}
	mu.Lock()
	defer mu.Unlock()
	log.Printf("группа: ID=%s, пользователь=%s, ", groupID, wuserID)

	var isUserFree bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)", wuserID).Scan(&isUserFree)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки группы пользователя"}`, http.StatusInternalServerError)
		return
	}
	if !isUserFree {
		json.NewEncoder(w).Encode(response{Message: "Пользователя нет в базе"})
		return
	}

	var isGroupExist bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1)", groupID).Scan(&isGroupExist)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки существования группы"}`, http.StatusInternalServerError)
		return
	}
	if !isGroupExist {
		json.NewEncoder(w).Encode(response{Message: "У вас нет такой группы"})
		return
	}

	var isLead bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1 AND g_lead = $2)", groupID, userID).Scan(&isLead)
	if err != nil {
		http.Error(w, `{"message":"Ошибка группы"}`, http.StatusInternalServerError)
		return
	}
	if !isLead {
		json.NewEncoder(w).Encode(response{Message: "У вас нет прав на это"})
		return
	}

	var isUserExist bool
	err := db.QueryRow("SELECT EXISTS ( SELECT 1 FROM users u JOIN w_groups g ON u.group_id = g.group_id WHERE g.g_name = $1 AND u.user_id = $2)", groupID, wuserID).Scan(&isUserExist)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки пользователя"}`, http.StatusInternalServerError)
		return
	}
	if !isUserExist {
		json.NewEncoder(w).Encode(response{Message: "У вас нет пользователя с таким ID в группе"})
		return
	}
	//

	_, err = db.Exec("UPDATE users SET group_id = NULL FROM w_groups WHERE users.group_id = w_groups.group_id AND w_groups.g_name = $1 AND users.user_id = $2;", groupID, wuserID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка удаления пользователя из группы"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(response{Message: "Пользователь удален из группы"})

	_, err = db.Exec("UPDATE users SET status = NULL WHERE user_id = $1;", wuserID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка изменения статуса"}`, http.StatusInternalServerError)
		return
	}

}

func handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
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
	groupName := r.Form.Get("leadText") // leadID - id пользователя, которого делаем лидом

	if userID == "" || groupName == "" {
		http.Error(w, `{"message":"не указан пользователь (отправитель или назначаемый)"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	var name bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1 )", groupName).Scan(&name)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки имени группы"}`, http.StatusInternalServerError)
		return
	}
	if !name {
		json.NewEncoder(w).Encode(response{Message: "Группы с таким именем не существует"})
		return
	}

	var isLead bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1 AND g_lead = $2)", groupName, userID).Scan(&isLead)
	if err != nil {
		http.Error(w, `{"message":"Ошибка группы"}`, http.StatusInternalServerError)
		return
	}
	if !isLead {
		json.NewEncoder(w).Encode(response{Message: "У вас нет прав на это"})
		return
	}
	_, err = db.Exec("DELETE FROM w_groups WHERE g_name = $1;", groupName)
	if err != nil {
		http.Error(w, `{"message":"Ошибка создания группы"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(response{Message: "Группа успешно удалена"})

}

func handleAddUser(w http.ResponseWriter, r *http.Request) {
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
	input := r.Form.Get("leadText")

	if userID == "" || input == "" {
		http.Error(w, `{"message":"не указан пользователь (отправитель или назначаемый)"}`, http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(input, " ", 2)
	var wuserID, groupID string
	if len(parts) >= 2 {
		wuserID = parts[0]
		groupID = parts[1]
	} else {
		// Если разделитель не найден, присваиваем весь текст первой переменной
		wuserID = input
		groupID = ""
	}
	mu.Lock()
	defer mu.Unlock()
	log.Printf("группа: ID=%s, пользователь=%s, ", groupID, wuserID)
	var isUserExist bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)", wuserID).Scan(&isUserExist)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки пользователя"}`, http.StatusInternalServerError)
		return
	}
	if !isUserExist {
		json.NewEncoder(w).Encode(response{Message: "У вас нет пользователя с таким ID"})
		return
	}

	var isUserFree bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND group_id IS NOT NULL)", wuserID).Scan(&isUserFree)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки группы пользователя"}`, http.StatusInternalServerError)
		return
	}
	if isUserFree {
		json.NewEncoder(w).Encode(response{Message: "Пользователь уже состоит в группе"})
		return
	}

	var isGroupExist bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1)", groupID).Scan(&isGroupExist)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки существования группы"}`, http.StatusInternalServerError)
		return
	}
	if !isGroupExist {
		json.NewEncoder(w).Encode(response{Message: "У вас нет такой группы"})
		return
	}

	var isGroupLead bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1 AND g_lead = $2)", groupID, userID).Scan(&isGroupLead)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки лида группы"}`, http.StatusInternalServerError)
		return
	}
	if !isGroupLead {
		json.NewEncoder(w).Encode(response{Message: "Вы не лид данной группы"})
		return
	}

	_, err = db.Exec("UPDATE Users SET group_id = ( SELECT group_id FROM w_groups WHERE g_name = $1) WHERE user_id = $2;", groupID, wuserID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка добавления пользователя в группу"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(response{Message: "Пользователь добавлен в группу"})

	_, err = db.Exec("UPDATE users SET status = 'worker' WHERE user_id = $1;", wuserID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка изменения статуса"}`, http.StatusInternalServerError)
		return
	}

}

func handleAddGroup(w http.ResponseWriter, r *http.Request) {
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
	groupName := r.Form.Get("leadText") // leadID - id пользователя, которого делаем лидом

	if userID == "" || groupName == "" {
		http.Error(w, `{"message":"не указан пользователь (отправитель или назначаемый)"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	groupID := uuid.New().String()
	log.Printf("Создана группа: ID=%s, Name=%s, Lead=%s\n", groupID, groupName, userID)
	// Назначаем пользователю статус "lead"
	var name bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM w_groups WHERE g_name = $1 )", groupName).Scan(&name)
	if err != nil {
		http.Error(w, `{"message":"Ошибка проверки имени группы"}`, http.StatusInternalServerError)
		return
	}
	if name {
		json.NewEncoder(w).Encode(response{Message: "Группа с таким именем уже есть"})
		return
	}
	_, err = db.Exec("INSERT INTO w_groups (group_id, g_name, g_lead) VALUES ($1, $2, $3)", groupID, groupName, userID)
	if err != nil {
		http.Error(w, `{"message":"Ошибка создания группы"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(response{Message: "Группа успешно создана"})

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
