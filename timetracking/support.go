package timetracking

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// parseFilter - парсинг фильтра
func parseFilter(filterS string) map[string]any {
	filter := map[string]any{}
	if len(filterS) != 0 {
		pairs := strings.Split(filterS, "%26%26")
		for _, pair := range pairs {
			parts := strings.Split(pair, "=")
			if len(parts) != 2 {
				continue
			}
			filter[parts[0]] = parts[1]
		}
	}

	return filter
}

// sendResponseOrError - обработка ошибок
// Если ошибки нет - возвращаем 200 и тело запроса или OK
// Если внутренняя ошибка - возвращаем 500 и текст ошибки
// Если ошибка - возвращаем 400 и текст ошибки
func sendResponseOrError(op string, err error, w http.ResponseWriter, body []byte, attr ...any) {
	if err == nil {
		slog.Debug(op+" success", attr...)
		if len(body) == 0 {
			body = []byte("OK")
		}
		w.Write(body)
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Info(op+" failed", append(attr, slog.String("error", err.Error()))...)

	if errors.Is(err, &InternalError{}) {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write([]byte(err.Error()))
	w.WriteHeader(http.StatusBadRequest)
}

func get[T any](fields map[string]any, name string) T {
	if v, ok := fields[name].(T); ok {
		return v
	}
	return *new(T)
}
