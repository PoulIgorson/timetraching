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
	const op = "TimeTrackingService: HandlerGetUser"

	slog.Info(op)

	pasportSeries := r.URL.Query().Get("pasportSeries")
	pasportNumber := r.URL.Query().Get("pasportNumber")

	user, err := h.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		sendResponseOrError(op, err, w, nil)
	}

	body, err := json.Marshal(user)
	sendResponseOrError("HandlerGetUser", err, w, body, slog.Any("user", user))
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

	filter := parseFilter(filterS)

	limit, _ := strconv.Atoi(limitS)
	offset, _ := strconv.Atoi(offsetS)

	slog.Debug("TimeTrackingService: HandlerGetUsers", slog.String("filterString", filterS), slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	users, err := h.FindUsersByFilter(filter, limit, offset)
	if err != nil {
		sendResponseOrError("HandlerGetUsers", err, w, nil)
		return
	}

	body, err := json.Marshal(map[string]any{
		"users": users,
	})
	sendResponseOrError("HandlerGetUsers", err, w, body, slog.String("users", fmt.Sprintf("%+t", users)))
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

	periodFrom, errFrom := time.Parse(time.DateTime, periodFromS)
	periodTo, errTo := time.Parse(time.DateTime, periodToS)

	if len(pasportSeries) == 0 || len(pasportNumber) == 0 {
		sendResponseOrError("HandlerCalculateCostByUser", &InvalidError{"invalid passport"}, w, nil)
		return
	}

	if errFrom != nil || errTo != nil {
		sendResponseOrError("HandlerCalculateCostByUser", errors.Join(&InvalidError{"invalid period"}, errFrom, errTo), w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerCalculateCostByUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Any("periodFrom", periodFrom), slog.Any("periodTo", periodTo))

	cost, err := h.CalculateCostByUser(pasportSeries, pasportNumber, periodFrom, periodTo)
	if err != nil {
		sendResponseOrError("HandlerCalculateCostByUser", err, w, nil)
		return
	}

	body, err := json.Marshal(map[string]any{
		"costs": cost,
	})
	sendResponseOrError("HandlerCalculateCostByUser", err, w, body, slog.Any("costs", cost))
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
	if err != nil {
		sendResponseOrError("HandlerBeginTaskForUser", err, w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerBeginTaskForUser", slog.String("body", string(body)))

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
		TaskId              int32  `json:"taskId"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		sendResponseOrError("HandlerBeginTaskForUser", err, w, body)
		return
	}

	slog.Debug("TimeTrackingService: HandlerBeginTaskForUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		sendResponseOrError("HandlerBeginTaskForUser", &InvalidError{"invalid passport"}, w, body)
		return
	}

	err = h.BeginTaskForUser(seriesNumber[0], seriesNumber[1], data.TaskId)
	sendResponseOrError("HandlerBeginTaskForUser", err, w, nil)
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
	const op = "TimeTrackingService: HandlerEndTaskForUser"

	slog.Info(op)

	body, err := io.ReadAll(r.Body)
	slog.Debug(op, slog.String("body", string(body)))
	if err != nil {
		sendResponseOrError(op, err, w, nil)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
		TaskId              int32  `json:"taskId"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		sendResponseOrError(op, err, w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerEndTaskForUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		sendResponseOrError(op, &InvalidError{"invalid passport"}, w, body)
		return
	}

	err = h.EndTaskForUser(seriesNumber[0], seriesNumber[1], data.TaskId)
	sendResponseOrError(op, err, w, nil)
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
	const op = "TimeTrackingService: HandlerDeleteUser"

	slog.Info(op)

	body, err := io.ReadAll(r.Body)

	slog.Debug(op, slog.String("body", string(body)))

	if err != nil {
		sendResponseOrError(op, err, w, nil)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		sendResponseOrError(op, err, w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerDeleteUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		sendResponseOrError(op, &InvalidError{"invalid passport"}, w, body)
		return
	}

	err = h.DeleteUser(seriesNumber[0], seriesNumber[1])
	sendResponseOrError(op, err, w, nil)
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
		sendResponseOrError("HandlerUpdateUser", err, w, nil)
		return
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		sendResponseOrError("HandlerUpdateUser", err, w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerUpdateUser unmarshaled data", slog.Any("data", data))

	pasportNumber, _ := data["pasportNumber"].(string)

	seriesNumber := strings.Split(pasportNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		sendResponseOrError("HandlerUpdateUser", &InvalidError{"invalid passport"}, w, body)
		return
	}

	delete(data, "pasportNumber")

	err = h.UpdateInfoUser(seriesNumber[0], seriesNumber[1], data)
	if err != nil {
		sendResponseOrError("HandlerUpdateUser", err, w, nil)
		return
	}

	sendResponseOrError("HandlerUpdateUser", err, w, nil)
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
		sendResponseOrError("HandlerCreateUser", err, w, nil)
		return
	}

	var data struct {
		PasportSeriesNumber string `json:"pasportNumber"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		sendResponseOrError("HandlerCreateUser", err, w, nil)
		return
	}

	slog.Debug("TimeTrackingService: HandlerCreateUser unmarshaled data", slog.Any("data", data))

	seriesNumber := strings.Split(data.PasportSeriesNumber, " ")
	if len(seriesNumber) != 2 || seriesNumber[0] == "" || seriesNumber[1] == "" {
		sendResponseOrError("HandlerCreateUser", &InvalidError{"invalid passport"}, w, body)
		return
	}

	newId, err := h.CreateUser(seriesNumber[0], seriesNumber[1])
	sendResponseOrError("HandlerCreateUser", err, w, []byte(fmt.Sprintf(`{"id": %d}`, newId)))
}
