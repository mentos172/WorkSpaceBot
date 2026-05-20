package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UserTaskState struct {
	IsRunning bool
	TaskName  string
}

var (
	trackerBaseURL  = "http://tracker:9000"
	businessBaseURL = "http://groupmanager:8000"
	awaitingInput   = make(map[string]string) // userID -> ожидаемый action ("start", "stop")
	userTasks       = make(map[string]UserTaskState)
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN не установлен в переменных окружения.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Обработка сообщений с текстом
		if update.Message != nil {
			userID := fmt.Sprint(update.Message.From.ID)
			username := update.Message.From.UserName

			// Проверяем, ожидаем ли мы ввод для этого пользователя
			if act, ok := awaitingInput[userID]; ok {
				inputText := update.Message.Text
				delete(awaitingInput, userID) // снимаем статус ожидания

				var res string
				if act == "start" {
					if state, exists := userTasks[userID]; exists && state.IsRunning {
						res = "Задача уже запущена."
					} else {
						res = callTrackerAPI("/start_task", userID, username, inputText)
						if strings.Contains(res, "время") {
							userTasks[userID] = UserTaskState{IsRunning: true, TaskName: inputText}
						}
					}
				} else if act == "stop" {
					if state, exists := userTasks[userID]; !exists || !state.IsRunning {
						res = "Нет активной задачи для остановки."
					} else {
						res = callTrackerAPI("/stop_task", userID, username, inputText)
						if !strings.Contains(res, "Ошибка") && !strings.HasPrefix(res, "Ошибка") {
							// При успешной остановке сбрасываем состояние
							userTasks[userID] = UserTaskState{IsRunning: false, TaskName: ""}
						}
					}
				} else if act == "lead" {
					res = callBusinessAPI("/add_lead", userID, username, inputText)
				} else if act == "add_group" {
					res = callBusinessAPI("/add_group", userID, username, inputText)
				} else if act == "add_user" {
					res = callBusinessAPI("/add_user", userID, username, inputText)
				} else if act == "delete_group" {
					res = callBusinessAPI("/delete_group", userID, username, inputText)
				} else if act == "delete_user" {
					res = callBusinessAPI("/delete_user", userID, username, inputText)
				} else {
					res = "Некорректный тип ожидаемого ввода."
				}
				sendMessage(bot, update.Message.Chat.ID, res)
				continue
			} else {
				switch update.Message.Text {
				case "/start":
					sendButtons(bot, update.Message.Chat.ID)
				default:
					sendMessage(bot, update.Message.Chat.ID, "Используйте команду /start или нажмите кнопки ниже.")
					sendButtons(bot, update.Message.Chat.ID) // Показываем кнопки при любом другом сообщении для удобства
				}
				continue
			}
		}

		// Обработка нажатий на Inline-кнопки
		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			userID := fmt.Sprint(update.CallbackQuery.From.ID)
			chatID := update.CallbackQuery.Message.Chat.ID

			var text string
			switch data {
			case "start_task":
				if state, exists := userTasks[userID]; exists && state.IsRunning {
					text = "Задача уже запущена."
				} else {
					sendMessage(bot, chatID, "Введите название задачи.")
					awaitingInput[userID] = "start"
					continue
				}
			case "stop_task":
				if state, exists := userTasks[userID]; !exists || !state.IsRunning {
					text = "Нет активной задачи для остановки."
				} else {
					sendMessage(bot, chatID, "Введите описание или комментарии к задаче.")
					awaitingInput[userID] = "stop"
					continue
				}
				// case "admin_console":
				// 	sendAdminButtons(bot, chatID)
				// 	return // завершить обработку здесь, чтобы не отправлять дублирующее сообщение

			case "admin_console":
				status, err := getUserStatus(userID)
				if err != nil {
					sendMessage(bot, chatID, "Ошибка при проверке прав.")
					return
				}
				if status == "lead" || status == "god" {
					sendAdminButtons(bot, chatID)
				} else {
					sendMessage(bot, chatID, "У вас нет прав для доступа к административной консоли.")
				}
				return

			case "add_group":
				sendMessage(bot, chatID, "Введите название группы, которую хотите создать.")
				awaitingInput[userID] = "add_group"
				continue
			case "add_user":
				sendMessage(bot, chatID, "Введите id пользователя и название группы через пробел.")
				awaitingInput[userID] = "add_user"
				continue
			case "delete_group":
				sendMessage(bot, chatID, "Введите название группы которую хотите удалить.")
				awaitingInput[userID] = "delete_group"
				continue
			case "delete_user":
				sendMessage(bot, chatID, "Введите id пользователя и название группы через пробел.")
				awaitingInput[userID] = "delete_user"
				continue
			case "add_lead":
				sendMessage(bot, chatID, "Введите id пользователя, которого хотите назначить лидом.")
				awaitingInput[userID] = "lead"
				continue
			case "my_id":
				text = userID
			default:
				text = "Неизвестная команда"
			}
			// Отвечаем на callback, чтобы убрать «часики» в интерфейсе
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
			if _, err := bot.Request(callback); err != nil {
				log.Println("Ошибка отправки callback:", err)
			}
			sendMessage(bot, chatID, text)
		}
	}
}

func sendButtons(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Старт", "start_task"),
			tgbotapi.NewInlineKeyboardButtonData("Стоп", "stop_task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ваш ID", "my_id"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Консоль администратора", "admin_console"),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки кнопок:", err)
	}
}

func sendAdminButtons(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите административную команду:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Добавить Группу", "add_group"),
			tgbotapi.NewInlineKeyboardButtonData("Добавить Пользователя", "add_user"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Удалить Пользователя", "delete_user"),
			tgbotapi.NewInlineKeyboardButtonData("Удалить Группу", "delete_group"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Добавить Лида", "add_lead"),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки admin-кнопок:", err)
	}
}

func callTrackerAPI(endpoint, userID, username string, extra ...string) string {
	fullURL := fmt.Sprintf("%s%s", trackerBaseURL, endpoint)
	client := http.Client{Timeout: 5 * time.Second}

	var resp *http.Response
	var err error

	if len(extra) > 0 && extra[0] != "" {
		data := url.Values{}
		data.Set("user_id", userID)
		data.Set("username", username)
		if endpoint == "/start_task" {
			data.Set("task", extra[0])
		} else if endpoint == "/stop_task" {
			data.Set("comment", extra[0])
		}
		resp, err = client.PostForm(fullURL, data)
	} else {
		fullURLWithQuery := fmt.Sprintf("%s?user_id=%s", fullURL, url.QueryEscape(userID))
		resp, err = client.Get(fullURLWithQuery)
	}

	if err != nil {
		return "Ошибка вызова сервиса трекера"
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "Задача уже запущена. Для остановки используйте кнопку / Стоп"
	}
	if resp.StatusCode == http.StatusNotFound {
		return "Нет запущенной задачи"
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Ошибка трекера: %s", resp.Status)
	}

	var respJSON map[string]string

	err = json.NewDecoder(resp.Body).Decode(&respJSON)
	if err != nil {
		return "Ошибка чтения ответа трекера"
	}

	if msg, ok := respJSON["message"]; ok {
		if duration, ok2 := respJSON["duration"]; ok2 {
			return fmt.Sprintf("%s, время: %s", msg, duration)
		}
		return msg
	}

	return "Непредвиденный ответ от трекера"
}

func callBusinessAPI(endpoint, userID, username, leadText string) string {
	fullURL := fmt.Sprintf("%s%s", businessBaseURL, endpoint)
	client := http.Client{Timeout: 5 * time.Second}

	data := url.Values{}
	data.Set("user_id", userID)
	data.Set("username", username)
	data.Set("leadText", leadText) // Передача текста лида

	resp, err := client.PostForm(fullURL, data)
	if err != nil {
		return "Ошибка вызова бизнес-сервиса"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Ошибка бизнес-сервиса: %s", resp.Status)
	}

	var respJSON map[string]string
	err = json.NewDecoder(resp.Body).Decode(&respJSON)
	if err != nil {
		return "Непредвиденный ответ от бизнес-сервиса"
	}

	if msg, ok := respJSON["message"]; ok {
		return msg
	}
	return "Ответ бизнес-сервиса без сообщения"
}

func getUserStatus(userID string) (string, error) {
	// пример вызова API
	apiURL := fmt.Sprintf("%s/get_user_status?user_id=%s", businessBaseURL, url.QueryEscape(userID))
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Status, nil
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки сообщения в Telegram:", err)
	}
}
