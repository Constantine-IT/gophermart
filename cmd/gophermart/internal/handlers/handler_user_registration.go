package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
)

//	User - структура для передачи информации о пользователях в виде JSON
//	используется в обработчиках UserAuthenticationHandler и UserRegistrationHandler
type User struct {
	UserID   string `json:"login"`    //  логин пользователя
	Password string `json:"password"` //  пароль пользователя (hash)
}

//	UserRegistrationHandler - обработчик регистрации нового пользователя
//	в случае успеха выдаёт пользователю cookie для дальнейшей авторизованной работы в системе

func (app *Application) UserRegistrationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	//	очищаем cookie с идентификатором сессии
	http.SetCookie(w, &http.Cookie{Name: "sessionid"})

	body, err := io.ReadAll(r.Body) // считываем JSON содержимое тела запроса
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println("JSON body read error:", err.Error())
		return
	}

	//	создаём экземпляр структуры для заполнения из JSON
	jsonUser := User{}

	//	парсим JSON из тела запроса и записываем результат в экземпляр структуры User
	err = json.Unmarshal(body, &jsonUser)
	//	проверяем успешно ли парсится JSON
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println("JSON body parsing error:", err.Error())
		return
	}

	//	создаём нового пользователя

	sessionID, err := app.Datasource.UserRegister(jsonUser.UserID, jsonUser.Password)
	if errors.Is(err, storage.ErrUserAlreadyExist) { //	если такой пользователь уже существует
		http.Error(w, "user with same login already exist", http.StatusConflict)
		return
	}
	if err != nil { //	при всех остальных ошибках при создании пользователя
		http.Error(w, "unable to create new user", http.StatusInternalServerError)
		app.ErrorLog.Println(err.Error())
		return
	}

	//	при успешном создании нового пользователя
	//	изготавливаем cookie "sessionid", со сроком жизни - 1 день
	cookie := &http.Cookie{
		Name: "sessionid", Value: sessionID, Expires: time.Now().AddDate(0, 0, 1),
	}
	//	вставляем cookie в response
	http.SetCookie(w, cookie)

	//	высылаем ответ со статусом 200
	w.WriteHeader(http.StatusOK)
}
