package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/theplant/luhn" //	алгоритм Луна для проверки корректности номера

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
)

//	PostUserOrderHandler - обработчик внесения нового заказа для начисления баллов
func (app *Application) PostUserOrderHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	sessionID, err := r.Cookie("sessionid") //	считываем идентификатор сессии из cookie запроса
	//	если идентификатор сессии отсутствует в cookie - пользователь не авторизован
	if err != nil || sessionID.Value == "" { // 		отвечаем со статусом 401
		http.Error(w, "please, authorise previously", http.StatusUnauthorized)
		return
	}

	order, err := io.ReadAll(r.Body) //	считываем номер заказа из тела запроса

	if err != nil { // при любых ошибках получения данных из запроса - отвечаем со статусом 400
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println(err.Error())
		return
	}

	orderNum, err := strconv.Atoi(string(order)) // конвертируем в целочисленный номер заказа
	//	проводим проверку номера заказа через алгоритм Луна
	if err != nil || !luhn.Valid(orderNum) { //	если номер заказа некорректный - отвечаем со статусом 422
		http.Error(w, "wrong order number format", http.StatusUnprocessableEntity)
		return
	}

	//	производим вставку нового номера заказа в базу для начисления баллов
	err = app.Datasource.OrderInsert(string(order), sessionID.Value)

	if errors.Is(err, storage.ErrOrderExistToAccount) { //	если такой заказ уже зарегистрирован ТЕКУЩИМ пользователем
		http.Error(w, err.Error(), http.StatusOK) // отвечаем со статусом 200
		return
	}
	if errors.Is(err, storage.ErrOrderExistToAnother) { //	если такой заказ уже зарегистрирован ДРУГИМ пользователем
		http.Error(w, err.Error(), http.StatusConflict) //	отвечаем со статусом 409
		return
	}
	if err != nil { //	при любых других ошибках при вставке заказа в базу
		http.Error(w, err.Error(), http.StatusInternalServerError) //	отвечаем со статусом 500
		return
	}

	//	если вставка прошла без ошибок - новый заказ принят в обработку
	w.WriteHeader(http.StatusAccepted) //	отвечаем со статусом 202
}
