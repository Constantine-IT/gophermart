package handlers

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
)

func TestHandlersResponse(t *testing.T) {

	type want struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name        string
		request     string
		requestType string
		body        string
		want        want
	}{
		{
			name:        "Test #1: user registration",
			request:     "/api/user/register",
			requestType: http.MethodPost,
			body:        `{"login": "test1", "password": "test1_password"}`,
			want: want{
				statusCode:  http.StatusOK,
				contentType: "",
				body:        "",
			},
		},
		{
			name:        "Test #2: user authorization",
			request:     "/api/user/login",
			requestType: http.MethodPost,
			body:        `{"login": "test1", "password": "test1_password"}`,
			want: want{
				statusCode:  http.StatusOK,
				contentType: "",
				body:        "",
			},
		},
		{
			name:        "Test #3: post order",
			request:     "/api/user/orders",
			requestType: http.MethodPost,
			body:        `2834832929383747`,
			want: want{
				statusCode:  http.StatusAccepted,
				contentType: "",
				body:        "",
			},
		},
		{
			name:        "Test #4: get balance",
			request:     "/api/user/balance",
			requestType: http.MethodGet,
			body:        "",
			want: want{
				statusCode:  http.StatusOK,
				contentType: "application/json",
				body:        `{"current":100, "withdrawn":0}`,
			},
		},
		{
			name:        "Test #5: post withdraw",
			request:     "/api/user/balance/withdraw",
			requestType: http.MethodPost,
			body:        `{"order": "2377225624", "sum": 11}`,
			want: want{
				statusCode:  http.StatusOK,
				contentType: "",
				body:        "",
			},
		},
		{
			name:        "Test #6: get orders",
			request:     "/api/user/orders",
			requestType: http.MethodGet,
			body:        "",
			want: want{
				statusCode:  http.StatusOK,
				contentType: "application/json",
				body:        `[{"accrual":100, "number":"2834832929383747", "status":"PROCESSED", "uploaded_at":"2022-01-01T00:00:00+03:00"}]`,
			},
		},
	}

	//	для тестов используется виртуальная база данных SQLlite в режиме "in memory"
	//	таблицы в ней создаются, как в настоящей базе, но изначально они пустые
	datasource, _ := storage.NewDatasource("")

	app := &Application{
		ErrorLog:       log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
		InfoLog:        log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		AccrualAddress: "", //	сервис для синхронизации заменяем заглушкой, переводящей все заказы в статус PROCESSED
		Datasource:     datasource,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := app.Routes()
			ts := httptest.NewServer(r)
			defer ts.Close()

			resp, body := testSimpleRequest(t, ts, tt.requestType, tt.request, tt.body, datasource)
			defer resp.Body.Close()
			assert.Equal(t, tt.want.statusCode, resp.StatusCode)
			assert.Equal(t, tt.want.contentType, resp.Header.Get("Content-Type"))
			if resp.Header.Get("Content-Type") == "application/json" {
				assert.JSONEq(t, tt.want.body, body)
			} else {
				assert.Equal(t, tt.want.body, body)
			}
		})
	}
}

func testSimpleRequest(t *testing.T, ts *httptest.Server, method, path string, body string, datasource storage.Datasource) (*http.Response, string) {

	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	require.NoError(t, err)

	//	для тестовой симуляции вычислим sessionID для пользователя с тестовым login/password
	sessionID, _ := datasource.UserAuthorise("test1", "test1_password")
	//	а также обновим статусы всех заказов в PROCESSED, с начислением 100 баллов
	datasource.UpdateOrdersStatus("")
	//	а ещё зададим cookie с названием sessionid и значением равным вычисленному sessionID
	req.AddCookie(&http.Cookie{
		Name: "sessionid", Value: sessionID,
	})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	return resp, string(respBody)
}
