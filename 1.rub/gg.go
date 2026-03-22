package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Хранение состояний пользователей для диалогов с ботом
var userStates = make(map[int64]string)

const (
	stateWaitingGroupName = "waiting_group_name"
	stateWaitingLeadData  = "waiting_lead_data"
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
		var chatID int64
		var userID string

		if update.Message != nil {
			chatID = update.Message.Chat.ID
			userID = fmt.Sprint(update.Message.From.ID)
		} else if update.CallbackQuery != nil {
			chatID = update.CallbackQuery.Message.Chat.ID
			userID = fmt.Sprint(update.CallbackQuery.From.ID)
		} else {
			continue
		}

		// Если пользователь в состоянии ввода группы или лида
		if state, ok := userStates[chatID]; ok && update.Message != nil && update.Message.Text != "" {
			text := update.Message.Text
			switch state {
			case stateWaitingGroupName:
				resp, err := CreateGroup(userID, text)
				if err != nil {
					sendMessage(bot, chatID, "Ошибка: "+err.Error())
				} else {
					sendMessage(bot, chatID, resp)
				}
				delete(userStates, chatID)
			case stateWaitingLeadData:
				resp, err := AddLead(userID, text)
				if err != nil {
					sendMessage(bot, chatID, "Ошибка: "+err.Error())
				} else {
					sendMessage(bot, chatID, resp)
				}
				delete(userStates, chatID)
			}
			continue
		}

		// Обработка обычных сообщений
		if update.Message != nil && update.Message.Text != "" {
			switch update.Message.Text {
			case "/start":
				sendButtons(bot, chatID)

			case "/stop":
				userID := fmt.Sprint(update.Message.From.ID)
				msg := callTrackerAPI("/stop", userID)
				sendMessage(bot, chatID, msg)

			// Добавляем обработку новых команд
			case "/create_group":
				userStates[chatID] = stateWaitingGroupName
				sendMessage(bot, chatID, "Введите название новой группы:")

			case "/add_lead":
				userStates[chatID] = stateWaitingLeadData
				sendMessage(bot, chatID, "Введите данные лида в формате: Имя, Телефон")

			default:
				sendMessage(bot, chatID, "Используйте команды /start, /stop, /create_group, /add_lead или нажмите кнопки.")
				sendButtons(bot, chatID)
			}
			continue
		}

		// Обработка callbackQuery
		if update.CallbackQuery != nil {
			data := update.CallbackQuery.Data
			var text string
			switch data {
			case "start":
				text = callTrackerAPI("/start", userID)
			case "stop":
				text = callTrackerAPI("/stop", userID)
			case "status":
				text = callTrackerAPI("/status", userID)
			case "create_group":
				userStates[chatID] = stateWaitingGroupName
				text = "Введите название новой группы:"
			case "add_lead":
				userStates[chatID] = stateWaitingLeadData
				text = "Введите данные лида в формате: Имя, Телефон"
			default:
				text = "Неизвестная команда"
			}

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
			if _, err := bot.Request(callback); err != nil {
				log.Println("Ошибка отправки callback:", err)
			}
			sendMessage(bot, chatID, text)
		}
	}
}

// Ваша функция отправки кнопок с добавлением новых кнопок
func sendButtons(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Старт", "start"),
			tgbotapi.NewInlineKeyboardButtonData("Стоп", "stop"),
			tgbotapi.NewInlineKeyboardButtonData("Статус", "status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Создать группу", "create_group"),
			tgbotapi.NewInlineKeyboardButtonData("Добавить лида", "add_lead"),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки кнопок:", err)
	}
}

// Заглушки для бизнес-логики и отправки сообщений, замените на вашу реализацию
func CreateGroup(userID, groupName string) (string, error) {
	// Ваша логика создания группы
	return "Группа '" + groupName + "' создана.", nil
}

func AddLead(userID, leadData string) (string, error) {
	// Ваша логика добавления лида
	return "Лид '" + leadData + "' добавлен.", nil
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Println("Ошибка отправки сообщения:", err)
	}
}

func callTrackerAPI(endpoint, userID string) string {
	// Ваша логика вызова API трекера
	return fmt.Sprintf("Вызов API %s для пользователя %s", endpoint, userID)
}

// 	for update := range updates {
// 		// Обработка сообщений с текстом
// 		if update.Message != nil {
// 			switch update.Message.Text {
// 			case "/start":
// 				sendButtons(bot, update.Message.Chat.ID)

// 			case "/stop":
// 				userID := fmt.Sprint(update.Message.From.ID)
// 				msg := callTrackerAPI("/stop", userID)
// 				sendMessage(bot, update.Message.Chat.ID, msg)

// 			default:
// 				sendMessage(bot, update.Message.Chat.ID, "Используйте команды /start и /stop или нажмите кнопки ниже.")
// 				sendButtons(bot, update.Message.Chat.ID) // Показываем кнопки при любом другом сообщении для удобства
// 			}
// 			continue
// 		}

// 		// Обработка нажатий на Inline-кнопки
// 		if update.CallbackQuery != nil {
// 			data := update.CallbackQuery.Data
// 			userID := fmt.Sprint(update.CallbackQuery.From.ID)
// 			chatID := update.CallbackQuery.Message.Chat.ID

// 			var text string
// 			switch data {
// 			case "start":
// 				text = callTrackerAPI("/start", userID)
// 			case "stop":
// 				text = callTrackerAPI("/stop", userID)
// 			case "status":
// 				text = callTrackerAPI("/status", userID)
// 			default:
// 				text = "Неизвестная команда"
// 			}

// 			// Отвечаем на callback, чтобы убрать «часики» в интерфейсе
// 			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, text)
// 			if _, err := bot.Request(callback); err != nil {
// 				log.Println("Ошибка отправки callback:", err)
// 			}

// 			// Отправляем результат в чат
// 			sendMessage(bot, chatID, text)
// 		}
// 	}
// }

// func sendButtons(bot *tgbotapi.BotAPI, chatID int64) {
// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Старт", "start"),
// 			tgbotapi.NewInlineKeyboardButtonData("Стоп", "stop"),
// 			tgbotapi.NewInlineKeyboardButtonData("Статус", "status"),
// 		),
// 	)
// 	if _, err := bot.Send(msg); err != nil {
// 		log.Println("Ошибка отправки кнопок:", err)
// 	}
// }
