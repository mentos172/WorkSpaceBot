package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	trackerBaseURL  = "http://tracker:9000"
	businessBaseURL = "http://groupmanager:8000"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")

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
			switch update.Message.Text {
			case "/start":
				sendButtons(bot, update.Message.Chat.ID)

			case "/stop":
				userID := fmt.Sprint(update.Message.From.ID)
				msg := callTrackerAPI("/stop", userID)
				sendMessage(bot, update.Message.Chat.ID, msg)

			default:
				sendMessage(bot, update.Message.Chat.ID, "Используйте команды /start и /stop или нажмите кнопки ниже.")
				sendButtons(bot, update.Message.Chat.ID) // Показываем кнопки при любом другом сообщении для удобства
			}
			continue
		}

		// Обработка нажатий на Inline-кнопки
		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			userID := fmt.Sprint(update.CallbackQuery.From.ID)
			chatID := update.CallbackQuery.Message.Chat.ID

			var text string
			switch data {
			case "start":
				text = callTrackerAPI("/start", userID)
			case "stop":
				text = callTrackerAPI("/stop", userID)
			case "task":
				text = callTrackerAPI("/task", userID)
			case "admin_console":
				text = callBusinessAPI("/admin_console", userID)
			default:
				text = "Неизвестная команда"
			}

			// Отвечаем на callback, чтобы убрать «часики» в интерфейсе
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
			if _, err := bot.Request(callback); err != nil {
				log.Println("Ошибка отправки callback:", err)
			}

			// Отправляем результат в чат
			sendMessage(bot, chatID, text)
		}
	}
}

func sendButtons(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Старт", "start"),
			tgbotapi.NewInlineKeyboardButtonData("Стоп", "stop"),
			tgbotapi.NewInlineKeyboardButtonData("Задача", "task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Консоль администратора", "admin_console"),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки кнопок:", err)
	}
}

func callTrackerAPI(endpoint, userID string) string {
	fullURL := fmt.Sprintf("%s%s?user_id=%s", trackerBaseURL, endpoint, url.QueryEscape(userID))
	client := http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(fullURL)
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
		// fallback — читаем как сырой текст (маловероятно)
		var buf = make([]byte, 512)
		n, _ := resp.Body.Read(buf)
		return string(buf[:n])
	}

	if msg, ok := respJSON["message"]; ok {
		if duration, ok2 := respJSON["duration"]; ok2 {
			return fmt.Sprintf("%s, время: %s", msg, duration)
		}
		return msg
	}

	return "Непредвиденный ответ от трекера"
}

func callBusinessAPI(endpoint, userID string) string {
	fullURL := fmt.Sprintf("%s%s?user_id=%s", businessBaseURL, endpoint, url.QueryEscape(userID))
	client := http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(fullURL)
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

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки сообщения в Telegram:", err)
	}
}
