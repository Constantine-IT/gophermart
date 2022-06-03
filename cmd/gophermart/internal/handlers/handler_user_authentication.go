package handlers

import (
	"encoding/json"
	"errors"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
	"io"
	"net/http"
	"time"
)

//	UserAuthenticationHandler - обработчик авторизации пользователя в системе
//	в случае успеха выдаёт пользователю cookie для дальнейшей авторизованной работы в системе
func (app *Application) UserAuthenticationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	//	очищаем cookie с идентификатором сессии
	http.SetCookie(w, &http.Cookie{Name: "sessionid"})

	body, err := io.ReadAll(r.Body) // считываем JSON содержимое тела запроса
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println("JSON body read error:", err.Error())
		return
	}

	jsonUser := User{} //	создаём экземпляр структуры для заполнения из JSON

	//	парсим JSON из тела запроса и записываем результат в экземпляр структуры User
	err = json.Unmarshal(body, &jsonUser)

	if err != nil { //	проверяем успешно ли парсится JSON
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println("JSON body parsing error:", err.Error())
		return
	}

	//	проверяем логин/пароль пользователя
	sessionID, err := app.Datasource.UserAuthorise(jsonUser.UserID, jsonUser.Password)
	if errors.Is(err, storage.ErrLoginPasswordIsWrong) { //	если логин/пароль не совпадают с зарегистрированными
		http.Error(w, "login or password is wrong", http.StatusUnauthorized)
		return
	}
	if err != nil { //	при всех остальных ошибках авторизации пользователя
		http.Error(w, "unable to authorise user", http.StatusInternalServerError)
		app.ErrorLog.Println(err.Error())
		return
	}

	//	при успешной авторизации пользователя, изготавливаем cookie "sessionid", со сроком жизни - 1 день
	cookie := &http.Cookie{
		Name: "sessionid", Value: sessionID, Expires: time.Now().AddDate(0, 0, 1),
	}
	//	вставляем cookie в response
	http.SetCookie(w, cookie)

	//	высылаем ответ со статусом 200
	w.WriteHeader(http.StatusOK)
}
