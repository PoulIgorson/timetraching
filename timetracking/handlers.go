package timetracking

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// SetupHandlers - настройка обработчиков
func (h *TimeTrackingService) SetupHandlers(group fiber.Router) {
	group.Get("/info", adaptor.HTTPHandlerFunc(h.HandlerGetUser))

	group.Get("/users", adaptor.HTTPHandlerFunc(h.HandlerGetUsers))

	group.Get("/calculate-cost-by-user", adaptor.HTTPHandlerFunc(h.HandlerCalculateCostByUser))

	group.Post("/begin-task-for-user", adaptor.HTTPHandlerFunc(h.HandlerBeginTaskForUser))

	group.Post("/end-task-for-user", adaptor.HTTPHandlerFunc(h.HandlerEndTaskForUser))

	group.Delete("/users", adaptor.HTTPHandlerFunc(h.HandlerDeleteUser))

	group.Put("/users", adaptor.HTTPHandlerFunc(h.HandlerUpdateUser))

	group.Post("/users", adaptor.HTTPHandlerFunc(h.HandlerCreateUser))
}

// HandlerGetUser - получение данных пользователя
// @Summary Get user data
// @Description Get user data by passport series and number
// @Tags User
// @Accept  json
// @Produce  json
// @Param   pasportSeries    query    string  true  "Passport series"
// @Param   pasportNumber    query    string  true  "Passport number"
// @Success 200 {object} User
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /info [get]
func (h *TimeTrackingService) HandlerGetUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerGetUser")

	pasportSeries := r.URL.Query().Get("pasportSeries")
	pasportNumber := r.URL.Query().Get("pasportNumber")

	slog.Debug("TimeTrackingService: HandlerGetUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	if pasportSeries == "" || pasportNumber == "" {
		slog.Info("TimeTrackingService: HandlerGetUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}

	user, err := h.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerGetUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(user) == 0 {
		slog.Debug("TimeTrackingService: HandlerGetUser failed", slog.String("error", "user not found"))
		http.Error(w, "user not found", http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerGetUser success", slog.Any("user", user))

	body, err := json.Marshal(user[0])
	if err != nil {
		slog.Info("TimeTrackingService: HandlerGetUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(body)
	w.WriteHeader(http.StatusOK)
}

// HandlerGetUsers - получение данных пользователей по фильтру и пагинации
// @Summary Get users data by filter and pagination
// @Description Get users data by filter and pagination
// @Tags User
// @Accept  json
// @Produce  json
// @Param   filter    query    string  false  "Filter"
// @Param   limit     query    int     false  "Limit"
// @Param   offset    query    int     false  "Offset"
// @Success 200 {array} User
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /users [get]
func (h *TimeTrackingService) HandlerGetUsers(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerGetUsers")

	filterS := r.URL.Query().Get("filter")
	limitS := r.URL.Query().Get("limit")
	offsetS := r.URL.Query().Get("offset")

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

	limit, _ := strconv.Atoi(limitS)
	offset, _ := strconv.Atoi(offsetS)

	slog.Debug("TimeTrackingService: HandlerGetUsers", slog.String("filterString", filterS), slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	users, err := h.FindUsersByFilter(filter, limit, offset)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerGetUsers failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(users) == 0 {
		slog.Info("TimeTrackingService: HandlerGetUsers failed", slog.String("error", "users not found"))
		http.Error(w, "users not found", http.StatusBadRequest)
		return
	}

	resp := map[string]any{
		"users": users,
	}

	body, err := json.Marshal(resp)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerGetUsers failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Debug("TimeTrackingService: HandlerGetUsers success")

	w.Write(body)
	w.WriteHeader(http.StatusOK)
}

// @Summary Затраты времени на задачи
// @Description Возвращает затраты времени на задачи по идентификатору пользователя
// @Tags Time Tracking
// @Accept json
// @Produce json
// @Param pasportSeries query string true "Серия паспорта"
// @Param pasportNumber query string true "Номер паспорта"
// @Param periodFrom    query string true "Начало периода (в формате ISO 8601)"
// @Param periodTo      query string true "Окончание периода (в формате ISO 8601)"
// @Success 200 {object} map[string]any "Список счетов (costs)"
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /calculate-cost-by-user [get]
func (h *TimeTrackingService) HandlerCalculateCostByUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerCalculateCostByUser")

	pasportSeries := r.URL.Query().Get("pasportSeries")
	pasportNumber := r.URL.Query().Get("pasportNumber")
	periodFromS := r.URL.Query().Get("periodFrom")
	periodToS := r.URL.Query().Get("periodTo")

	if len(periodFromS) < len(time.DateTime) {
		periodFromS = periodFromS + " 00:00:00"
	}
	if len(periodToS) < len(time.DateTime) {
		periodToS = periodToS + " 23:59:59"
	}

	periodFrom, _ := time.Parse(time.DateTime, periodFromS)
	periodTo, _ := time.Parse(time.DateTime, periodToS)

	slog.Debug("TimeTrackingService: HandlerCalculateCostByUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Any("periodFrom", periodFrom), slog.Any("periodTo", periodTo))

	cost, err := h.CalculateCostByUser(pasportSeries, pasportNumber, periodFrom, periodTo)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerCalculateCostByUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := map[string]any{
		"costs": cost,
	}

	body, err := json.Marshal(resp)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerCalculateCostByUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Debug("TimeTrackingService: HandlerCalculateCostByUser success")

	w.Write(body)
	w.WriteHeader(http.StatusOK)
}

// HandlerBeginTaskForUser - начать отсчет времени по задаче
// @Summary Begin task for user
// @Description Begin task for user
// @Tags Time Tracking
// @Accept json
// @Produce json
// @Param pasportNumber body string true "Passport number"
// @Param taskId        body string true "Task ID"
// @Success 200 {string} string "OK"
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /begin-task-for-user [post]
func (h *TimeTrackingService) HandlerBeginTaskForUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerBeginTaskForUser")

	body, err := io.ReadAll(r.Body)

	slog.Debug("TimeTrackingService: HandlerBeginTaskForUser", slog.String("body", string(body)))

	if err != nil {
		slog.Info("TimeTrackingService: HandlerBeginTaskForUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
		TaskId              int32  `json:"taskId"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerBeginTaskForUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerBeginTaskForUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		slog.Info("TimeTrackingService: HandlerBeginTaskForUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	err = h.BeginTaskForUser(seriesNumber[0], seriesNumber[1], data.TaskId)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerBeginTaskForUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerBeginTaskForUser success")

	w.Write([]byte("OK"))
	w.WriteHeader(http.StatusOK)
}

// HandlerEndTaskForUser - закончить отсчет времени по задаче
// @Summary Finish task time tracking
// @Description Finish tracking time for a specific task
// @Produce json
// @Param pasportNumber body string true "Passport number"
// @Param taskId        body string true "Task ID"
// @Success 200 {string} string "OK"
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /end-task-for-user [post]
func (h *TimeTrackingService) HandlerEndTaskForUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerEndTaskForUser")

	body, err := io.ReadAll(r.Body)

	slog.Debug("TimeTrackingService: HandlerEndTaskForUser", slog.String("body", string(body)))

	if err != nil {
		slog.Info("TimeTrackingService: HandlerEndTaskForUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
		TaskId              int32  `json:"taskId"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerEndTaskForUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerEndTaskForUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		slog.Info("TimeTrackingService: HandlerEndTaskForUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	err = h.EndTaskForUser(seriesNumber[0], seriesNumber[1], data.TaskId)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerEndTaskForUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerEndTaskForUser success")

	w.Write([]byte("OK"))
	w.WriteHeader(http.StatusOK)
}

// HandlerDeleteUser - удалить пользователя
// @Summary Delete user
// @Description Delete user by passport series and number
// @Tags User
// @Accept  json
// @Produce  json
// @Param   pasportSeries    query    string  true  "Passport series"
// @Param   pasportNumber    query    string  true  "Passport number"
// @Success 200 {string} string "OK"
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /users [delete]
func (h *TimeTrackingService) HandlerDeleteUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerDeleteUser")

	body, err := io.ReadAll(r.Body)

	slog.Debug("TimeTrackingService: HandlerDeleteUser", slog.String("body", string(body)))

	if err != nil {
		slog.Info("TimeTrackingService: HandlerDeleteUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerDeleteUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerDeleteUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		slog.Info("TimeTrackingService: HandlerDeleteUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	err = h.DeleteUser(seriesNumber[0], seriesNumber[1])
	if err != nil {
		slog.Info("TimeTrackingService: HandlerDeleteUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerDeleteUser success")

	w.Write([]byte("OK"))
	w.WriteHeader(http.StatusOK)
}

// HandlerUpdateUser - обновить данные пользователя
// @Summary Update user data
// @Description Update user data by passport series and number
// @Tags User
// @Accept  json
// @Produce  json
// @Param   pasportSeries   query     string  true  "Passport series"
// @Param   pasportNumber   query     string  true  "Passport number"
// @Param   body            body  User    true  "User data"
// @Success 200 {string} string "OK"
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /users [put]
func (h *TimeTrackingService) HandlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerUpdateUser")

	body, err := io.ReadAll(r.Body)

	slog.Debug("TimeTrackingService: HandlerUpdateUser", slog.String("body", string(body)))

	if err != nil {
		slog.Info("TimeTrackingService: HandlerUpdateUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data map[string]any

	err = json.Unmarshal(body, &data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerUpdateUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerUpdateUser unmarshaled data", slog.Any("data", data))

	pasportNumber, _ := data["pasportNumber"].(string)

	seriesNumber := strings.Split(pasportNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		slog.Info("TimeTrackingService: HandlerUpdateUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	delete(data, "pasportNumber")

	err = h.UpdateInfoUser(seriesNumber[0], seriesNumber[1], data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerUpdateUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerUpdateUser success")

	w.Write([]byte("OK"))
	w.WriteHeader(http.StatusOK)
}

// HandlerCreateUser - создание пользователя
// @Summary Create user
// @Description Create user with passport data
// @Tags User
// @Accept  json
// @Produce  json
// @Param   body     body    User   true        "User data"
// @Success 200 {int32} int32 0
// @Failure 400 {string} error "Неверные параметры запроса"
// @Failure 500 {string} error "Внутренняя ошибка сервера"
// @Router /users [post]
func (h *TimeTrackingService) HandlerCreateUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("TimeTrackingService: HandlerCreateUser")

	body, err := io.ReadAll(r.Body)

	slog.Debug("TimeTrackingService: HandlerCreateUser", slog.String("body", string(body)))

	if err != nil {
		slog.Info("TimeTrackingService: HandlerCreateUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		slog.Info("TimeTrackingService: HandlerCreateUser failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerCreateUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		slog.Info("TimeTrackingService: HandlerCreateUser failed", slog.String("error", "invalid pasport number"))
		http.Error(w, "invalid pasport number", http.StatusBadRequest)
		return
	}

	newId, err := h.CreateUser(seriesNumber[0], seriesNumber[1])
	if err != nil {
		slog.Info("TimeTrackingService: HandlerCreateUser failed", slog.String("error", err.Error()))

		if errors.As(err, &ErrInternal) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("TimeTrackingService: HandlerCreateUser success")

	w.Write([]byte(fmt.Sprintf(`{"id": %d}`, newId)))
	w.WriteHeader(http.StatusOK)
}
