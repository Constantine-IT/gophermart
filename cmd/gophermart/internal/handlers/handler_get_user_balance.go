package handlers

import (
	"encoding/json"
	"net/http"
)

//	GetUserBalanceHandler - обработчик запроса баланса счёта пользователя в системе
func (app *Application) GetUserBalanceHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	sessionID, err := r.Cookie("sessionid") //	считываем идентификатор сессии из cookie запроса
	//	если идентификатор сессии отсутствует в cookie - пользователь не авторизован
	if err != nil || sessionID.Value == "" { // 		отвечаем со статусом 401
		http.Error(w, "please, authorise previously", http.StatusUnauthorized)
		return
	}

	//	производим запрос баланса баллов данного пользователя
	accrualSum, withdrawSum, err := app.Datasource.GetBalance(sessionID.Value)
	if err != nil { //											при любых ошибках запроса баланса
		http.Error(w, err.Error(), http.StatusInternalServerError) //	отвечаем со статусом 500
		return
	}

	//	описываем структуру для отправки данных о балансе счёта пользователя в JSON виде
	type balance struct {
		Current   float32 `json:"current"`
		Withdrawn float32 `json:"withdrawn"`
	}

	//	создаём экземпляр структуры balance
	userBalance := balance{
		Current:   accrualSum,
		Withdrawn: withdrawSum,
	}

	//	кодируем информацию в JSON
	body, err := json.Marshal(userBalance)
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
