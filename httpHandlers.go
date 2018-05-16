package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// registrationHandler - обработчик, который осуществляет регистрацию нового пользователя.
func registrationHandler(w http.ResponseWriter, r *http.Request) {
	user := getAndCheckUser(w, r)
	if user == nil {
		return
	}

	fd, _, err := r.FormFile("profileImage")
	if err == nil {
		user.ProfileImage = true
	}

	var id string // проверка в бд есть ли такой логин
	err = db.QueryRow("SELECT id FROM users WHERE login = $1", user.Login).Scan(&id)
	if err == nil {
		log.Printf("Инфо. Попытка зарегистрироваться с логином которые уже существует(%s)", user.Login)
		http.Error(w, "Пользователь с таким логином уже существует.", http.StatusBadRequest)
		return
	}

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Ошибка. При проверке в БД пользователя с именем %s: %v\n", user.Login, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	var userid int // регистрация в бд
	err = db.QueryRow(`INSERT INTO users(login, password, name, lastName, phone, profileImage)
	VALUES($1, $2, $3, $4, $5, $6) RETURNING id`, user.Login, user.Password, user.Name, user.LastName, user.Phone, user.ProfileImage).Scan(&userid)
	if err != nil {
		log.Printf("Ошибка. При добавлении нового пользователя в БД: %v\n", err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	if user.ProfileImage {
		err = saveFile(fd, strconv.Itoa(userid))
		if err != nil {
			http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
			return
		}
	}

	w.Write([]byte("Регистрация успешно выполнена"))
	log.Printf("Инфо. Пользователь с ником %q зарегистрировался", user.Login)
}

// authorizationHandler - обработчик, который осуществляет авторизация пользователя. Токен записывается в cookie.
func authorizationHandler(w http.ResponseWriter, r *http.Request) {
	login := strings.ToLower(r.FormValue("login"))
	estimatePass := r.FormValue("password")

	if len(estimatePass) < 6 {
		http.Error(w, "Длина пароля должна быть не менее 7 символов.", http.StatusBadRequest)
		return
	}

	if login == "" {
		http.Error(w, "Логин не может быть пустой строкой.", http.StatusBadRequest)
		return
	}

	var password string
	var id string

	err := db.QueryRow("SELECT id, password FROM users WHERE login = $1", login).Scan(&id, &password)
	if err != nil && err == sql.ErrNoRows {
		log.Println("Инфо. Запрос по несуществующему пользователю(login - " + login + " ): " + err.Error())
		http.Error(w, "Пользователя с таким логином не существует.", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Println("Ошибка. При поиске в БД пользователя(логин - " + login + " ): " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	estimatePass = generateMD5hash(estimatePass)

	if password != estimatePass {
		http.Error(w, "Неверный пароль.", http.StatusBadRequest)
		return
	}

	token := generateToken()

	_, err = db.Exec(`INSERT INTO authorizations VALUES($1, $2)`, id, token)
	if err != nil {
		log.Println("Ошибка. При создании записи в БД об авторизации пользователя: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "token", Value: token})

	log.Println("Инфо. Пользователь " + login + " авторизовался.")
	w.Write([]byte("Авторизация успешно выполнена."))
}

// logOutHandler - реализует выход из аккаунта
func logOutHandler(w http.ResponseWriter, r *http.Request) {
	token := getTokenFromCookie(w, r)
	if token == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{Name: "token", Expires: time.Now().UTC()})

	_, err := db.Exec(`DELETE FROM authorizations WHERE token = $1`, token)
	if err != nil {
		log.Println("Ошибка. При удалении записи в БД об авторизации пользователя: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Выход из системы успешно выполнен."))
}

// profileImageHandler - возвращает аватарку, если она есть.
func profileImageHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	var withProfileImage bool

	err := db.QueryRow(`SELECT profileimage FROM users WHERE id=$1`, id).Scan(&withProfileImage)
	if err == sql.ErrNoRows {
		log.Println("Инфо. Запрос по несуществующему пользователю(ид - " + id + " ): " + err.Error())
		http.Error(w, "Запрашиваемого пользователя не существует.", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Println("Ошибка. При запросе информации о пользователе (id - " + id + " ): " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	if withProfileImage {
		http.ServeFile(w, r, fmt.Sprintf("%s%s", storageDirectory, id))
	} else {
		http.Error(w, "У данного пользоватля нет аватарки.", http.StatusBadRequest)
	}
}

// profileHandler - отдает информацию о пользователе.
func profileInfoHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	user := User{}

	err := db.QueryRow(`SELECT name, lastname, phone, profileimage FROM users WHERE id=$1`, id).
		Scan(&user.Name, &user.LastName, &user.Phone, &user.ProfileImage)

	if err == sql.ErrNoRows {
		log.Println("Инфо. Запрос по несуществующему пользователю(ид - " + id + " ): " + err.Error())
		http.Error(w, "Запрашиваемого пользователя не существует.", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Println("Ошибка. При поиске в БД информации о пользователе(ид - " + id + " ): " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(user)
	if err != nil {
		log.Println("Ошибка. При маршалинге в json результата: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json;")

	_, err = w.Write(data)
	if err != nil {
		log.Println("Ошибка. При отдачи метоинформации: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
	}

	log.Println("Инфо. Отдача метаданных профиля id = " + id + ")успешно закончена")
}

// registrationHandler - обработчик, который осуществляет регистрацию нового пользователя.
func addCarHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	car := getAndCheckCar(w, r)
	if car == nil {
		return
	}

	_, err := db.Exec("INSERT INTO cars(brand, model, vin, year, userid) VALUES($1, $2, $3, $4, $5)", car.Brand, car.Model, car.VIN, car.Year, id)
	if err != nil {
		log.Printf("Ошибка. При добавлении в БД машины пользователю(ид = %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	log.Printf("Инфо. Пользователю (ид = %s) добавлена машина", id)
	w.Write([]byte("Машина успешно добавлена успешно выполнена"))
}

func GetCarsHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	rows, err := db.Query("SELECT id, brand, model, vin, year, userid FROM cars WHERE userid = $1 AND deleted = FALSE", id)

	if err != nil {
		log.Printf("Ошибка. При выборке из БД информации о машинах пользователя(ид =  %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make([]*Car, 0)

	for rows.Next() {
		car := Car{}
		err = rows.Scan(&car.ID, &car.Brand, &car.Model, &car.VIN, &car.Year, &car.UserID)
		if err != nil {
			log.Printf("Ошибка. При выборке из БД информации о машинах пользователя(ид =  %s): %s\n", id, err.Error())
			http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
			return
		}
		result = append(result, &car)
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Println("Ошибка. При маршалинге в json результата: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json;")

	_, err = w.Write(data)
	if err != nil {
		log.Println("Ошибка. При отдачи метоинформации: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
	}

	log.Println("Инфо. Отдача информации о машинах пользователя(ид =  " + id + ") успешно закончена")

}

func removeCarHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	carID := r.FormValue("id")

	if carID == "" {
		log.Println("Ошибка. Пользователь не передал id машины для удаления машины.")
		http.Error(w, "Необходимо передать id машины, которую вы хотите удалить", http.StatusBadRequest)
		return
	}
	_, err := db.Exec(`UPDATE cars SET deleted = TRUE WHERE id = $1`, carID)
	if err != nil {
		log.Println("Ошибка. При удалении записи в БД об машине: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Машина успешно удалена из системы."))
}

func addOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	order := getAndCheckOrder(w, r)
	if order == nil {
		return
	}
	order.UserID = id
	order.Status = StatusOpen

	_, err := db.Exec("INSERT INTO orders(status, date, cost, carID, userID, info) VALUES($1, $2, $3, $4, $5, $6)",
		order.Status,
		order.GetFormatDate(),
		order.Cost,
		order.CarID,
		order.UserID,
		order.Info,
	)
	if err != nil {
		log.Printf("Ошибка. При добавлении в БД заказа пользователю(ид = %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	log.Printf("Инфо. Пользователю (ид = %s) добавлен заказ", id)
	w.Write([]byte("Заказ успешно добавлен"))
}

// getOrdersHandler - отдает все заказы пользователя в формате json
func getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	var err error
	var rows *sql.Rows
	if r.FormValue("isclosed") == "true" {
		rows, err = db.Query("SELECT * FROM orders WHERE userid = $1 AND status = $2 ORDER BY status, date", id, StatusClosed)
	} else {
		rows, err = db.Query("SELECT * FROM orders WHERE userid = $1 AND status != $2 ORDER BY status, date", id, StatusClosed)
	}
	if err != nil {
		log.Printf("Ошибка. При выборке из БД информации о заказах пользователя(ид =  %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make([]*Order, 0)
	var (
		brand, model, year string
		date               time.Time
	)

	for rows.Next() {
		order := Order{}
		err = rows.Scan(&order.ID, &order.Status, &date, &order.Cost, &order.CarID, &order.UserID, &order.Info, &order.IsNewMSGForUser)
		if err != nil {
			log.Printf("Ошибка. При выборке из БД информации о машинах пользователя(ид =  %s): %s\n", id, err.Error())
			http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
			return
		}

		order.Month = strconv.Itoa(int(date.Month()))
		order.Day = strconv.Itoa(date.Day())
		order.Year = strconv.Itoa(date.Year())

		err = db.QueryRow("SELECT brand, model, year FROM cars WHERE id = $1", order.CarID).Scan(&brand, &model, &year)
		if err != nil {
			log.Printf("Ошибка. При выборке из БД информации о машине пользователя(id машины =  %s): %s\n", order.CarID, err.Error())
			http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
			return
		}

		order.CarInfo = brand + " " + model + "(" + year + ")"
		result = append(result, &order)
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Println("Ошибка. При маршалинге в json результата: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json;")

	_, err = w.Write(data)
	if err != nil {
		log.Println("Ошибка. При отдачи метоинформации: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
	}

	log.Println("Инфо. Отдача информации о заказах пользователя(ид =  " + id + ") успешно закончена")

}

// addMessageToOrderHandler - добавляет сообщение к заказу
func addMessageToOrderHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	message := getAndCheckMessage(w, r)
	if message == nil {
		return
	}

	var status int
	err := db.QueryRow(`SELECT status FROM orders WHERE id = $1 AND userID = $2 LIMIT 1`, message.OrderID, id).Scan(&status)
	if err == sql.ErrNoRows {
		log.Println("Инфо. Попытка добавить сообщение к указанному заказу: " + err.Error())
		http.Error(w, "Невозможно добавить сообщение к указанному заказу.", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println("Ошибка. При поиске записи в БД о заказе: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	if status == StatusClosed {
		log.Println("Инфо. Попытка добавить сообщение к закрытому заказу: ")
		http.Error(w, "Невозможно добавить сообщение к закрытому заказу.", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO messages(isadmin, date, text, orderid) VALUES(FALSE, $1, $2, $3)", message.Date, message.Text, message.OrderID)
	if err != nil {
		log.Printf("Ошибка. При добавлении сообщения к заказу(ид пользователя =  %s,ид заказа =  %s ): %s\n", id, message.OrderID, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}
}

func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	id := checkAuthorization(w, r)

	if id == "" {
		return
	}

	orderID := r.FormValue("orderID")
	if _, err := strconv.Atoi(orderID); err != nil {
		log.Println("Инфо. Попытка получить сообщения заказа с невалидным ID.")
		http.Error(w, "Ошибка. Получен некорректный номер заказа", http.StatusBadRequest)
		return
	}

	err := db.QueryRow(`SELECT id FROM orders WHERE id = $1 AND userID = $2 LIMIT 1`, orderID, id).Scan(&orderID)
	if err == sql.ErrNoRows {
		log.Println("Инфо. Попытка получить сообщения заказа, которого нет у пользователя или его вовсе не существует: " + err.Error())
		http.Error(w, "Укажите верный номер заказа.", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println("Ошибка. При поиске записи в БД о заказе: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(`SELECT * FROM messages WHERE orderid = $1 ORDER BY date`, orderID)
	if err != nil {
		log.Printf("Ошибка. При выборке из БД информации о машинах пользователя(ид =  %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже."+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make([]*Message, 0)

	for rows.Next() {
		message := Message{}
		err = rows.Scan(&message.IsAdmin, &message.Date, &message.Text, &message.OrderID)
		if err != nil {
			log.Printf("Ошибка. При выборке из БД информации о сообщениях заказа(ид =  %s): %s\n", orderID, err.Error())
			http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
			return
		}
		result = append(result, &message)
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Println("Ошибка. При маршалинге в json результата: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json;")

	_, err = w.Write(data)
	if err != nil {
		log.Println("Ошибка. При отдачи метоинформации: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
	}

	db.Exec("UPDATE orders SET newmsgforuser=FALSE WHERE id=$1", orderID)

	log.Println("Инфо. Отдача информации о сообщения заказа(ид =  " + orderID + ") успешно закончена")

}

// addMessageToOrderHandler - добавляет сообщение к заказу
func addAdminMeassageHandler(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	orderID := r.FormValue("orderID")
	if text == "" || orderID == "" {
		return
	}

	_, err := db.Exec("INSERT INTO messages(isadmin, date, text, orderid) VALUES(TRUE, $1, $2, $3)", time.Now(), text, orderID)
	if err != nil {
		log.Printf("Ошибка. При добавлении сообщения админа к заказу(ид заказа =  %s ): %s\n", orderID, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}
	db.Exec("UPDATE orders SET newmsgforuser=TRUE WHERE id=$1", orderID)
}
