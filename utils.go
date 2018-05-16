package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/STEJLS/ServiceStation/XMLconfig"
	_ "github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

// InitFlags - инициализирует флаги командной строки
func InitFlags() {
	flag.StringVar(&logSource, "log_source", "log.txt", "Source for log file")
	flag.StringVar(&configSource, "config_source", "config.xml", "Source for config file")
	flag.Parse()
}

// InitLogger - настраивает логгер
func InitLogger() *os.File {
	logfile, err := os.OpenFile(logSource, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalln("Ошибка. Файл логов (%q) не открылся: ", logSource, err)
	}

	log.SetOutput(logfile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return logfile
}

// connectToDB - устанавливет соединение с БД и инициализирует глобальные переменные
func connectToDB(dbinfo XMLconfig.DataBase) {
	var err error
	connStr := "user=" + dbinfo.User + " password=" + dbinfo.Password + " dbname=" + dbinfo.DBname +
		" host=" + dbinfo.Host + " port=" + strconv.Itoa(dbinfo.Port) + " sslmode=" + dbinfo.SSLmode
	if db == nil {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Fatalln(fmt.Sprintf("Фатал. При подключении к серверу БД(%v:%v): ", dbinfo.Host, dbinfo.Port) + err.Error())
		}
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln("Фатал. При пинге сервера БД: " + err.Error())
	}

	log.Printf("Инфо. Подключение к базе данных установлено.")
}

// saveFile - сохраняет копию обрабатываемой песни на диске под именем newFileName,
// если все прошло успешно, то возвращает nil, в противном случае объект ошибки
func saveFile(readSeeker io.ReadSeeker, newFileName string) error {
	_, err := readSeeker.Seek(0, os.SEEK_SET)
	if err != nil {
		log.Println("Ошибка. Выход из запроса: не удалось перейти на начало копируемого файла: " + err.Error())
		return err
	}
	data, err := ioutil.ReadAll(readSeeker)
	if err != nil {
		log.Println("Ошибка. Выход из запроса: не удалось прочитать файл пользователя: " + err.Error())
		return err
	}

	err = ioutil.WriteFile(storageDirectory+newFileName, data, 0666)
	if err != nil {
		log.Println("Ошибка. Выход из запроса: не удалось создать новый файл: " + err.Error())
		return err
	}

	log.Println("Инфо. Файл сохранен на диске.")
	return nil
}

// removeFile - удаляет файл и обрабатывает возможные ошибки
func removeFile(fileName string) {
	err := os.Remove(fileName)
	if err != nil {
		log.Println("Ошибка. При удалении файла: " + err.Error())
	}
}

//ValidateUser - проверяет поступившие данные о пользователе на бизнес правила
func ValidateUser(u *User, regexpForPhone *regexp.Regexp) string {
	if len(u.Login) < 6 {
		return "Ошибка. Длина логина меньше 6 символов."
	}
	if len(u.Password) < 6 {
		return "Ошибка. Длина пароля меньше 6 символов."
	}
	if len(u.Name) == 0 {
		return "Ошибка. Поле имя не может быть пустым."
	}
	if len(u.LastName) == 0 {
		return "Ошибка. Поле фамилия не может быть пустым."
	}

	if regexpForPhone.MatchString(u.Phone) {
		//return "Некорректный номер телефона"
	}

	return ""
}

//ValidateMessage - проверяет поступившие данные о сообщения на бизнес правила
func ValidateMessage(message *Message) string {

	if _, err := strconv.Atoi(message.OrderID); err != nil {
		return "Ошибка. Неверный id заказа."
	}

	if message.Text == "" {
		return "Ошибка. Пустой текст сообщения."
	}

	return ""
}

//ValidateOrder - проверяет поступившие данные о заказе на бизнес правила
func ValidateOrder(order *Order) string {
	day, err := strconv.Atoi(order.Day)
	if err != nil {
		return "Ошибка. Неверная дата."
	}

	if day > 31 || day < 1 {
		return "Ошибка. Неверная дата."
	}

	month, err := strconv.Atoi(order.Month)
	if err != nil {
		return "Ошибка. Неверная дата."
	}

	if month > 12 || day < 1 {
		return "Ошибка. Неверная дата."
	}

	year, err := strconv.Atoi(order.Year)
	if err != nil {
		return "Ошибка. Неверная дата."
	}

	if year > 2100 || year < 2018 {
		return "Ошибка. Неверная дата."
	}

	if order.CarID == "" {
		return "Ошибка. Укажите машину"
	}

	if order.Info == "" {
		return "Ошибка. Информацию по заказу"
	}

	return ""
}

// ValidateCar - проверяет поступившие данные о машине на бизнес правила
func ValidateCar(car *Car) string {
	if car.Brand == "" {
		return "Ошибка. Необходимо указать марку машины."
	}

	if len(car.Brand) > 50 {
		return "Ошибка. Марка машины не может быть длиннее 20 символов."
	}

	if car.Model == "" {
		return "Ошибка. Необходимо указать модель машины."
	}

	if len(car.Model) > 20 {
		return "Ошибка. Модель машины не может быть длиннее 50 символов."
	}

	if len(car.VIN) != 17 {
		return "Ошибка. Получен некорректный VIN код."
	}

	if _, err := strconv.Atoi(car.Year); err != nil {
		return "Ошибка. Получен некорректный год."
	}

	return ""
}

// generateToken - генерирует уникальный токен для авторизации.
func generateToken() string {
	token, err := uuid.NewV4()

	if err != nil {
		log.Println("Ошибка. При генерации токена: " + err.Error())
	}

	return token.String()
}

// generateMD5hash - хэширует пароль по правилу: md5( md5(password) + salt)
func generateMD5hash(password string) string {

	md5hash := md5.Sum([]byte(password))

	temp := make([]byte, 0, md5.Size+len(salt))
	for _, item := range md5hash {
		temp = append(temp, item)
	}
	for _, item := range salt {
		temp = append(temp, item)
	}

	hasher := md5.New()
	hasher.Write(temp)
	return hex.EncodeToString(hasher.Sum(nil))
}

// getAndCheckUser - получает данные о новом пользователе из запроса,
// а так же валидирует параметры и солит пароль.
func getAndCheckUser(w http.ResponseWriter, r *http.Request) *User {
	user := NewUser(strings.ToLower(r.FormValue("login")), r.FormValue("password"), r.FormValue("name"), r.FormValue("lastName"), r.FormValue("phone"))
	re, err := regexp.Compile(`^((?:(?:\(?(?:|\+)([1-4]\d\d|[1-9]\d?)\)?)?[\-\.\\\/]?)?((?:\(?\d{1,}\)?[\-\.\\\/]?){0,})(\d+))$`)
	if err != nil {
		log.Printf("Ошибка. При компиляции регулярного выражения: %v\n", err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return nil
	}

	resultOfValidation := ValidateUser(user, re)
	if resultOfValidation != "" {
		http.Error(w, resultOfValidation, http.StatusBadRequest)
		return nil
	}

	user.Password = generateMD5hash(user.Password)

	return user
}

// getAndCheckUser - получает данные о новом пользователе из запроса,
// а так же валидирует параметры.
func getAndCheckCar(w http.ResponseWriter, r *http.Request) *Car {
	car := &Car{
		Brand: r.FormValue("brand"),
		Model: r.FormValue("model"),
		VIN:   r.FormValue("vin"),
		Year:  r.FormValue("year"),
	}

	resultOfValidation := ValidateCar(car)
	if resultOfValidation != "" {
		log.Println("Инфо. Попытка добавить машину с невалидными данными: " + resultOfValidation)
		http.Error(w, resultOfValidation, http.StatusBadRequest)
		return nil
	}

	return car
}

// getAndCheckOrder - получает данные о новом заказе из запроса,
// а так же валидирует параметры.
func getAndCheckOrder(w http.ResponseWriter, r *http.Request) *Order {

	order := &Order{
		Cost:  r.FormValue("cost"),
		Info:  r.FormValue("textInfo"),
		Month: r.FormValue("month"),
		Day:   r.FormValue("day"),
		Year:  r.FormValue("year"),
		CarID: r.FormValue("carID"),
	}

	resultOfValidation := ValidateOrder(order)
	if resultOfValidation != "" {
		log.Println("Инфо. Попытка добавить заказ с невалидными данными: " + resultOfValidation)
		http.Error(w, resultOfValidation, http.StatusBadRequest)
		return nil
	}

	return order

}

// getAndCheckOrder - получает данные о новом заказе из запроса,
// а так же валидирует параметры.
func getAndCheckMessage(w http.ResponseWriter, r *http.Request) *Message {
	message := &Message{
		IsAdmin: false,
		Date:    time.Now(),
		Text:    r.FormValue("text"),
		OrderID: r.FormValue("orderID"),
	}

	resultOfValidation := ValidateMessage(message)
	if resultOfValidation != "" {
		log.Println("Инфо. Попытка добавить сообщение с невалидными данными: " + resultOfValidation)
		http.Error(w, resultOfValidation, http.StatusBadRequest)
		return nil
	}

	return message

}

// checkAuthorization - проверяет cookie с токеном,
// и в случае его наличия возвращает логин пользовател.
func checkAuthorization(w http.ResponseWriter, r *http.Request) string {
	token := getTokenFromCookie(w, r)

	if token == "" {
		return ""
	}

	var id string
	err := db.QueryRow(`SELECT userid FROM authorizations WHERE token = $1`, token).Scan(&id)

	if err == sql.ErrNoRows {
		log.Println("Инфо. Попытка доступа по недействительному токену: " + err.Error())
		http.Error(w, "Устаревший токен авторизации.", http.StatusBadRequest)
		return ""
	}

	if err != nil {
		log.Println("Ошибка. При поиске записи в БД об авторизации пользователя: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return ""
	}

	return id
}

// getTokenFromCookie - возвращает token авторизации из cookie.
func getTokenFromCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("token")

	if err != nil && err == http.ErrNoCookie {
		http.Error(w, "Для начала работы необходимо авторизоваться.", http.StatusBadRequest)
		return ""
	}

	if err != nil {
		log.Println("Ошибка. При чтении cookie: " + err.Error())
		http.Error(w, "Неполадки на сервере, повторите попытку позже.", http.StatusInternalServerError)
		return ""
	}

	if cookie.Value == "" {
		http.Error(w, "Для начала работы необходимо авторизоваться.", http.StatusBadRequest)
		return ""
	}

	return cookie.Value
}
