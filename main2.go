package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Добавляем структуру для хранения состояния задач
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
		if update.Message != nil {
			userID := fmt.Sprint(update.Message.From.ID)
			username := update.Message.From.UserName

			// Проверка, ожидаем ли мы ввод
			if act, ok := awaitingInput[userID]; ok {
				inputText := update.Message.Text
				delete(awaitingInput, userID)

				var res string
				if act == "start" {
					// Проверка, запущена ли уже задача
					if state, exists := userTasks[userID]; exists && state.IsRunning {
						res = "Задача уже запущена."
					} else {
						res = callTrackerAPI("/start_task", userID, username, inputText)
						if strings.Contains(res, "время") {
							userTasks[userID] = UserTaskState{IsRunning: true, TaskName: inputText}
						}
					}
				} else if act == "stop" {
					// Проверка, есть ли запущенная задача
					if state, exists := userTasks[userID]; !exists || !state.IsRunning {
						res = "Нет активной задачи для остановки."
					} else {
						res = callTrackerAPI("/stop_task", userID, username, inputText)
						if !strings.Contains(res, "Ошибка") && !strings.HasPrefix(res, "Ошибка") {
							// При успешной остановке сбрасываем состояние
							userTasks[userID] = UserTaskState{IsRunning: false, TaskName: ""}
						}
					}
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
					sendButtons(bot, update.Message.Chat.ID)
				}
				continue
			}
		}

		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			userID := fmt.Sprint(update.CallbackQuery.From.ID)
			chatID := update.CallbackQuery.Message.Chat.ID

			var text string
			switch data {
			case "start_task":
				// Перед запросом проверяем, есть ли активная задача
				if state, exists := userTasks[userID]; exists && state.IsRunning {
					text = "Задача уже запущена."
				} else {
					sendMessage(bot, chatID, "Введите название задачи.")
					awaitingInput[userID] = "start"
					continue
				}
			case "stop_task":
				// Перед запросом проверяем, есть ли активная задача
				if state, exists := userTasks[userID]; !exists || !state.IsRunning {
					text = "Нет активной задачи для остановки."
				} else {
					sendMessage(bot, chatID, "Введите описание или комментарии к задаче.")
					awaitingInput[userID] = "stop"
					continue
				}
			case "admin_console":
				sendAdminButtons(bot, chatID)
				return
			case "add_group":
				text = "add group"
			case "add_user":
				text = "add user"
			case "delete_group":
				text = "delete group"
			case "add_lead":
				text = "add lead"
			default:
				text = "Неизвестная команда"
			}

			// Отправляем ответ callback, чтобы убрать "часики"
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
			if _, err := bot.Request(callback); err != nil {
				log.Println("Ошибка отправки callback:", err)
			}
			sendMessage(bot, chatID, text)
		}
	}
}
