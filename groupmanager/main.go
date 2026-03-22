package main

import "errors"

var (
	ErrEmptyGroupName  = errors.New("название группы не может быть пустым")
	ErrInvalidLeadData = errors.New("некорректные данные лида")
)

// Для простоты используем мапу userID->группы и лиды
var groups = make(map[string][]string)
var leads = make(map[string][]string)

// Создать новую группу
func CreateGroup(userID, groupName string) (string, error) {
	if groupName == "" {
		return "", ErrEmptyGroupName
	}
	groups[userID] = append(groups[userID], groupName)
	return "Группа '" + groupName + "' создана", nil
}

// Добавить лида: формат данных — "Имя, Телефон"
func AddLead(userID, leadData string) (string, error) {
	parts := []rune(leadData)
	if len(parts) == 0 {
		return "", ErrInvalidLeadData
	}
	// Можно добавить более строгую проверку
	leads[userID] = append(leads[userID], leadData)
	return "Лид '" + leadData + "' добавлен", nil
}
