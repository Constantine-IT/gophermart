package handlers

import (
	"encoding/json"
	"errors"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
	"net/http"
)

//	GetUserOrdersHandler - обработчик заявок на выдачу списка заказов пользователя для начисление баллов
func (app *Application) GetUserOrdersHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	sessionID, err := r.Cookie("sessionid") //	считываем идентификатор сессии из cookie запроса
	//	если идентификатор сессии отсутствует в cookie - пользователь не авторизован
	if err != nil || sessionID.Value == "" { // 		отвечаем со статусом 401
		http.Error(w, "please, authorise previously", http.StatusUnauthorized)
		return
	}

	//	производим запрос списка заказов для начисления баллов, сформированного данным пользователем
	orders, err := app.Datasource.GetOrders(sessionID.Value)

	if errors.Is(err, storage.ErrNoDataToAnswer) { //		если список заказов пуст
		http.Error(w, err.Error(), http.StatusNoContent) // отвечаем со статусом 204
		return
	}
	if err != nil { //													при любых других ошибках
		http.Error(w, err.Error(), http.StatusInternalServerError) //	отвечаем со статусом 500
		return
	}

	//	кодируем информацию в JSON
	body, err := json.Marshal(orders)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		app.ErrorLog.Println(err.Error())
		return
	}

	// Изготавливаем и возвращаем ответ, вставляя список заказов в тело ответа в JSON виде
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) //	отвечаем со статусом 200
	w.Write(body)                //	пишем JSON в тело ответа
}
