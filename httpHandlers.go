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

	rows, err := db.Query("SELECT * FROM cars WHERE userid = $1", id)

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

	_, err := db.Exec(`DELETE FROM cars WHERE id = $1`, carID)
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

	test := order.GetFormatDate()

	_, err := db.Exec("INSERT INTO orders(status, date, cost, carID, userID, info) VALUES($1, $2, $3, $4, $5, $6)",
		order.Status,
		test,
		order.Cost,
		order.CarID,
		order.UserID,
		order.Info,
	)

	//_, err := db.Exec("INSERT INTO orders(status, date, cost, carID, userID, info) VALUES('1', '11-20-2014', '1000', '10', '2', 'fbdsbfiudsbfiudshfidsbif')")
	if err != nil {
		log.Printf("Ошибка. При добавлении в БД заказа пользователю(ид = %s): %s\n", id, err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return
	}

	log.Printf("Инфо. Пользователю (ид = %s) добавлен заказ", id)
	w.Write([]byte("Заказ успешно добавлен"))
}
