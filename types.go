package main

import (
	"time"
)

//User - структура, писывающая сущность пользователя.
type User struct {
	ID           uint64
	Login        string
	Password     string
	Name         string
	LastName     string
	Phone        string
	ProfileImage bool
}

//NewUser - Конструктор для нового объекта пользователя.
func NewUser(login string, pass string, name string, lastName string, Phone string) *User {
	return &User{
		ID:           0,
		Login:        login,
		Password:     pass,
		Name:         name,
		LastName:     lastName,
		Phone:        Phone,
		ProfileImage: false,
	}
}

//Car - структура, писывающая сущность автомобиля.
type Car struct {
	ID     string
	Brand  string
	Model  string
	VIN    string
	Year   string
	UserID string
}

//Order - структура, писывающая сущность заказа.
type Order struct {
	ID              string
	Status          int
	CarID           string
	CarInfo         string
	Month           string
	Day             string
	Year            string
	Cost            string
	Info            string
	UserID          string
	IsNewMSGForUser bool
}

// GetFormarDate - возвращает дату в формате мм-дд-гггг
func (order *Order) GetFormatDate() string {
	var result string
	if len(order.Month) == 1 {
		result += "0"
	}
	result += order.Month + "-"

	if len(order.Day) == 1 {
		result += "0"
	}

	return result + order.Day + "-" + order.Year
}

//Message - структура, писывающая сущность сообщения в заказе.
type Message struct {
	IsAdmin bool
	Date    time.Time
	Text    string
	OrderID string
}
