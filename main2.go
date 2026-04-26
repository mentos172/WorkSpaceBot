package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	trackerBaseURL  = "http://tracker-service:9000"
	businessBaseURL = "http://business-service:9001"
)

func main() {
	bot, err := tgbotapi.NewBotAPI("ВАШ_ТОКЕН_ТЕЛЕГРАМ")
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Обработка текстовых сообщений (команды /start, /stop)
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
				sendButtons(bot, update.Message.Chat.ID)
			}
			continue
		}

		// Обработка нажатий на inline-кнопки
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
			case "status":
				text = callTrackerAPI("/status", userID)
			case "add_group":
				text = callBusinessAPI("/add_group", userID)
			case "add_lead":
				text = callBusinessAPI("/add_lead", userID)
			default:
				text = "Неизвестная команда"
			}

			// Отвечаем callback-ом, чтобы убрать "загрузку" кнопки
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
			if _, err := bot.Request(callback); err != nil {
				log.Println("Ошибка отправки callback:", err)
			}

			// Отправляем сообщение с результатом действия
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
			tgbotapi.NewInlineKeyboardButtonData("Статус", "status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Добавить группу", "add_group"),
			tgbotapi.NewInlineKeyboardButtonData("Добавить лида", "add_lead"),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки кнопок:", err)
	}
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки сообщения:", err)
	}
}

func callTrackerAPI(endpoint, userID string) string {
	fullURL := fmt.Sprintf("%s%s?user_id=%s", trackerBaseURL, endpoint, url.QueryEscape(userID))
	client := http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(fullURL)
	if err != nil {
		return "Ошибка вызова трекера"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Ошибка трекера: %s", resp.Status)
	}

	var respJSON map[string]string
	err = json.NewDecoder(resp.Body).Decode(&respJSON)
	if err != nil {
		return "Непредвиденный ответ от трекера"
	}

	if msg, ok := respJSON["message"]; ok {
		return msg
	}
	return "Ответ трекера без сообщения"
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
