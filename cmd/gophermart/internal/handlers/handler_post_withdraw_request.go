package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
)

//	PostWithdrawRequestHandler - обработчик внесения заявки на списание баллов в счёт нового заказа
func (app *Application) PostWithdrawRequestHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	sessionID, err := r.Cookie("sessionid") //	считываем идентификатор сессии из cookie запроса
	//	если идентификатор сессии отсутствует в cookie - пользователь не авторизован
	if err != nil || sessionID.Value == "" { // 		отвечаем со статусом 401
		http.Error(w, "please, authorise previously", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body) //	считываем информации о заявке из тела запроса

	if err != nil { // при любых ошибках получения данных из запроса - отвечаем со статусом 400
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println(err.Error())
		return
	}

	//	описываем структуру для приема заявки в JSON виде
	type withdrawOrder struct {
		Order string  `json:"order"`
		Sum   float32 `json:"sum"`
	}
	//	создаём экземпляр структуры withdrawOrder
	withdrawIn := withdrawOrder{}

	//	парсим JSON и записываем результат в withdrawIn
	err = json.Unmarshal(body, &withdrawIn)

	if err != nil { //	проверяем успешно ли парсится JSON
		http.Error(w, err.Error(), http.StatusBadRequest)
		app.ErrorLog.Println("JSON body parsing error:", err.Error())
		return
	}

	// проверяем, конвертируется ли считанный номер заказа в целочисленное значение
	_, err = strconv.ParseInt(withdrawIn.Order, 10, 64)

	if err != nil { //	если номер заказа не является набором цифр - отвечаем со статусом 422
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		app.ErrorLog.Println(err.Error())
		return
	}

	//	производим вставку новой заявки на списание баллов в базу
	err = app.Datasource.WithdrawRequest(withdrawIn.Order, withdrawIn.Sum, sessionID.Value)
	
	if errors.Is(err, storage.ErrInsufficientFundsToAccount) { //	если на счёте недостаточно средств
		http.Error(w, err.Error(), http.StatusPaymentRequired) // отвечаем со статусом 402
		return
	}
	if err != nil { //							при любых других ошибках при вставке заказа в базу
		http.Error(w, err.Error(), http.StatusInternalServerError) //	отвечаем со статусом 500
		return
	}

	//	если вставка прошла без ошибок - баллы списаны в счёт заказа
	w.WriteHeader(http.StatusOK) //	отвечаем со статусом 200
}
