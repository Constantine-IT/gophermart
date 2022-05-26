package handlers

import (
	"encoding/json"
	"errors"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
	"net/http"
)

//	GetUserWithdrawalsHandler - обработчик заявок списание баллов в счёт новых заказов
func (app *Application) GetUserWithdrawalsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	sessionID, err := r.Cookie("sessionid") //	считываем идентификатор сессии из cookie запроса
	//	если идентификатор сессии отсутствует в cookie - пользователь не авторизован
	if err != nil || sessionID.Value == "" { // 		отвечаем со статусом 401
		http.Error(w, "please, authorise previously", http.StatusUnauthorized)
		return
	}

	//	производим запрос списка заявок на списание баллов, сформированного данным пользователем
	withdrawals, err := app.Datasource.GetWithdrawals(sessionID.Value)
	if errors.Is(err, storage.ErrNoDataToAnswer) { //		если список заявок пуст
		http.Error(w, err.Error(), http.StatusNoContent) // отвечаем со статусом 204
		return
	}
	if err != nil { //													при любых других ошибках
		http.Error(w, err.Error(), http.StatusInternalServerError) //	отвечаем со статусом 500
		return
	}

	//	кодируем информацию в JSON
	body, err := json.Marshal(withdrawals)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		app.ErrorLog.Println(err.Error())
		return
	}

	// Изготавливаем и возвращаем ответ, вставляя список заявок в тело ответа в JSON виде
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) //	отвечаем со статусом 200
	w.Write(body)                //	пишем JSON в тело ответа
}
