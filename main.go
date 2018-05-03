package main

import (
	"fmt"
	"log"
	"net/http"
	//go run main.go httpHandlers.go types.go utils.go  globals.go
	//go build main.go httpHandlers.go types.go utils.go  globals.go
	//pgx
	"github.com/STEJLS/ServiceStation/XMLconfig"
)

func main() {
	InitFlags()
	logFile := InitLogger()
	defer logFile.Close()

	config := XMLconfig.Get(configSource)

	connectToDB(config.Db)
	defer db.Close()

	server := http.Server{
		Addr: fmt.Sprintf("%v:%v", config.HTTP.Host, config.HTTP.Port),
	}

	http.HandleFunc("/registration", registrationHandler)
	http.HandleFunc("/authorization", authorizationHandler)
	http.HandleFunc("/profileInfo", profileInfoHandler)
	http.HandleFunc("/profileImage", profileImageHandler)
	http.HandleFunc("/addCar", addCarHandler)
	http.HandleFunc("/removeCar", removeCarHandler)
	http.HandleFunc("/logOut", logOutHandler)
	http.HandleFunc("/getCars", GetCarsHandler)
	http.HandleFunc("/addOrder", addOrderHandler)
	http.HandleFunc("/getOrders", getOrdersHandler)

	err := server.ListenAndServe()
	if err != nil {
		log.Println(err.Error())
	}
}
