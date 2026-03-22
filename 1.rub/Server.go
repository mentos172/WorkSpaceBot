package main

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Структура для хранения состояния пользователя
type UserTracker struct {
	StartTime   time.Time
	Accumulated time.Duration
	Tracking    bool
}

var users = make(map[int]*UserTracker)

func main() {
	bot, err := tgbotapi.NewBotAPI("8656289848:AAH7WbX4O3kF2l8jearYz6i3t0SPX_C84EA")
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Запущен как @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, _ := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			userID := update.Message.From.ID
			chatID := update.Message.Chat.ID
			text := update.Message.Text

			switch text {
			case "/start":
				// Отправляем сообщение с кнопками
				sendKeyboard(bot, chatID)
			case "/start_tracking":
				handleStart(bot, chatID, userID)
			case "/stop_tracking":
				handleStop(bot, chatID, userID)
			case "/status":
				handleStatus(bot, chatID, userID)
			default:
				// Обработка команд или сообщений
				// Можно также проверить, есть ли нажатие кнопки
				if text == "Начать" {
					handleStart(bot, chatID, userID)
				} else if text == "Стоп" {
					handleStop(bot, chatID, userID)
				} else if text == "Время" {
					handleStatus(bot, chatID, userID)
				} else {
					msg := tgbotapi.NewMessage(chatID, "Команда не распознана. Используйте /start или нажимайте кнопки.")
					bot.Send(msg)
				}
			}
		}

		// Обработка нажатий inline-кнопок, если есть
		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			chatID := update.CallbackQuery.Message.Chat.ID
			userID := update.CallbackQuery.From.ID

			if data == "start" {
				handleStart(bot, chatID, userID)
			} else if data == "stop" {
				handleStop(bot, chatID, userID)
			} else if data == "time" {
				handleStatus(bot, chatID, userID)
			}
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "Обработано"))
		}
	}
}

// Функция для отправки клавиатуры
func sendKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Начать"),
			tgbotapi.NewKeyboardButton("Стоп"),
			tgbotapi.NewKeyboardButton("Время"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Выберите опцию:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Остальные функции без изменений
func handleStart(bot *tgbotapi.BotAPI, chatID int64, userID int) {
	user, exists := users[userID]
	if !exists {
		user = &UserTracker{}
		users[userID] = user
	}

	if user.Tracking {
		bot.Send(tgbotapi.NewMessage(chatID, "Уже отслеживаешь."))
		return
	}

	user.StartTime = time.Now()
	user.Tracking = true
	bot.Send(tgbotapi.NewMessage(chatID, "Отслеживание началось!"))
}

func handleStop(bot *tgbotapi.BotAPI, chatID int64, userID int) {
	user, exists := users[userID]
	if !exists || !user.Tracking {
		bot.Send(tgbotapi.NewMessage(chatID, "Ты еще не начал отслеживать."))
		return
	}

	elapsed := time.Since(user.StartTime)
	user.Accumulated += elapsed
	user.Tracking = false
	bot.Send(tgbotapi.NewMessage(chatID, "Отслеживание остановлено!\nОбщее время: "+formatDuration(user.Accumulated)))
}

func handleStatus(bot *tgbotapi.BotAPI, chatID int64, userID int) {
	user, exists := users[userID]
	if !exists {
		bot.Send(tgbotapi.NewMessage(chatID, "Ты еще не начал отслеживать время."))
		return
	}

	var total time.Duration
	if user.Tracking {
		total = user.Accumulated + time.Since(user.StartTime)
	} else {
		total = user.Accumulated
	}
	bot.Send(tgbotapi.NewMessage(chatID, "Общее отслеженное время: "+formatDuration(total)))
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d: %02d: %02d", h, m, s)
}
