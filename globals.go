package main

import (
	"database/sql"
	"sync"
)

// logFileName - имя файла для логов, задается через флаг командной строки
var logSource string

// ConfigSource - имя файла для конфига, задается через флаг командной строки
var configSource string

//db - глобальная переменная подключения к Бд
var db *sql.DB

// sessions - карта для авторизации пользователей. Ключ токен, а значение - логин.
var sessions = make(map[string]string)

// lock - Мьютекс для корректной параллельной работы с картой sessions.
var lock = new(sync.RWMutex)

// salt - соль для пароля.
var salt = [12]byte{152, 123, 2, 1, 6, 84, 216, 35, 140, 158, 69, 128}

const (
	formFileName     string = "file"             // имя файла в форме на сайте
	storageDirectory string = "./profileImages/" // место для хранения песен
	StatusOpen       int    = 1                  // Открыт
	StatusСonfirmed  int    = 2                  // Подтвержден
	StatusClosed     int    = 3                  // Закрыт
)
