package timetracking

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	. "timetracking/storage"
)

var Logger = slog.Default()

// Структура пользователя
type User struct {
	Id            int32  `json:"-"`
	PasportSeries string `json:"-"`                    // серия паспорта
	PasportNumber string `json:"-"`                    // номер паспорта
	Surname       string `json:"surname"`              // фамилия
	Name          string `json:"name"`                 // имя
	Patronymic    string `json:"patronymic,omitempty"` // отчество
	Address       string `json:"address"`              // адрес
}

func NewUser(data map[string]any) *User {
	return &User{
		Id:            get[int32](data, "id"),
		PasportSeries: get[string](data, "pasport_series"),
		PasportNumber: get[string](data, "pasport_number"),
		Surname:       get[string](data, "surname"),
		Name:          get[string](data, "name"),
		Patronymic:    get[string](data, "patronymic"),
		Address:       get[string](data, "address"),
	}
}

type Task struct {
	Id          int32     `json:"-"`
	Title       string    `json:"title"`       // название
	Description string    `json:"description"` // описание
	PeriodFrom  time.Time `json:"periodFrom"`  // начало периода
	PeriodTo    time.Time `json:"periodTo"`    // конец периода

	UserId   int32         `json:"userId"`   // идентификатор пользователя
	Cost     time.Duration `json:"cost"`     // потраченное время
	WorkFrom time.Time     `json:"WorkFrom"` // время начала работы
}

func NewTask(data map[string]any) *Task {
	return &Task{
		Id:          get[int32](data, "id"),
		Title:       get[string](data, "title"),
		Description: get[string](data, "description"),
		PeriodFrom:  get[time.Time](data, "period_from"),
		PeriodTo:    get[time.Time](data, "period_to"),

		UserId:   get[int32](data, "user_id"),
		Cost:     time.Duration(get[int64](data, "cost")),
		WorkFrom: get[time.Time](data, "work_from"),
	}
}

// Сервис
type TimeTrackingService struct {
	storage Storage // интерфейс подключения к базе данных
}

// Конструктор
func NewTimeTrackingService(storage Storage) *TimeTrackingService {
	return &TimeTrackingService{
		storage: storage,
	}
}

func processStorageError(op string, err error, needLog bool) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		if needLog {
			slog.Info(op + " not found")
		}
		return &NotFoundError{"not found"}
	}

	if needLog {
		slog.Info(op + " failed")
	}
	return errors.Join(&StorageError{}, err)
}

// Методы

// Находит пользователя по паспорту
func (s *TimeTrackingService) FindUserByPassport(pasportSeries, pasportNumber string) (*User, error) {
	slog.Debug("TimeTrackingService: FindUserByPassport", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	if pasportSeries == "" || pasportNumber == "" {
		return nil, &InvalidError{"pasportSeries or pasportNumber is empty"}
	}

	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}

	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		slog.Info("TimeTrackingService: FindUserByPassport failed", slog.String("error", err.Error()))
		return nil, err
	}

	if len(user) == 0 {
		slog.Info("TimeTrackingService: FindUserByPassport failed", slog.String("error", "user not found"))
		return nil, &NotFoundError{"user not found"}
	}

	return user[0], nil
}

// Находит пользователей по фильтру с пагинацией, возвращает список пользователей
// Если не находит записей возвращает ErrNoRows
func (s *TimeTrackingService) FindUsersByFilter(filter map[string]any, limit, offset int) ([]*User, error) {
	const op = "TimeTrackingService: FindUsersByFilter"

	Logger.Debug(op, slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	// Получение пользователей по фильтру с пагинацией
	reader, err := s.storage.Select(UserCollection, filter, limit, offset)
	if err != nil {
		return nil, processStorageError(op, err, true)
	}

	// Чтение пользователей
	var users []*User
	for reader.Next() {
		record, err := reader.Read()
		if err := processStorageError(op, err, true); err != nil {
			return nil, err
		}

		users = append(users, NewUser(record.Fields))
	}

	Logger.Debug("TimeTrackingService: FindUsersByFilter users found", slog.Int("count", len(users)))
	return users, nil
}

// Находит задач по фильтру с пагинацией, возвращает список задач
// Если не находит записей возвращает ErrNoRows
func (s *TimeTrackingService) FindTasksByFilter(filter map[string]any, limit, offset int) ([]*Task, error) {
	const op = "TimeTrackingService: FindTasksByFilter"

	Logger.Debug(op, slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	// Получение задач по фильтру с пагинацией
	reader, err := s.storage.Select(TaskCollection, filter, limit, offset)
	if err != nil {
		return nil, processStorageError(op, err, true)
	}

	// Чтение задач
	var tasks []*Task
	for reader.Next() {
		record, err := reader.Read()
		if err := processStorageError(op, err, true); err != nil {
			return nil, err
		}

		tasks = append(tasks, NewTask(record.Fields))
	}

	Logger.Debug("TimeTrackingService: FindTasksByFilter tasks found", slog.Int("count", len(tasks)))
	return tasks, nil
}

// Вычисляет стоимость задачи по идентификатору пользователя
func (s *TimeTrackingService) CalculateCostByUser(pasportSeries, pasportNumber string, begin, end time.Time) ([]string, error) {
	const op = "TimeTrackingService: CalculateCostByUser"

	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		return nil, processStorageError(op, err, false)
	}

	Logger.Debug("TimeTrackingService: CalculateCostByUser user found", slog.Int("user", int(user.Id)))

	// Получение задач пользователя
	filter := map[string]any{
		"user_id": user.Id,
	}
	reader, err := s.storage.Select(TaskCollection, filter, 0, 0)
	if err != nil {
		return nil, processStorageError(op, err, true)
	}

	// Подсчет затраченного времени
	var costs []string
	for reader.Next() {
		record, err := reader.Read()
		if err := processStorageError(op, err, true); err != nil {
			return nil, err
		}

		periodFrom := get[time.Time](record.Fields, "period_from")
		periodTo := get[time.Time](record.Fields, "period_to")

		if periodTo.Before(begin) || periodFrom.After(end) {
			continue
		}

		costs = append(costs, fmt.Sprintf("%d-%v", record.Id, time.Duration(get[int64](record.Fields, "cost")).Truncate(time.Second)))
	}

	sort.Slice(costs, func(i, j int) bool {
		costI := strings.Split(costs[i], "-")
		costJ := strings.Split(costs[j], "-")
		return costI[1] > costJ[1]
	})

	Logger.Debug("TimeTrackingService: CalculateCostByUser cost calculated", slog.Int("userId", int(user.Id)), slog.Any("costs", costs))
	return costs, nil
}

// Запуск задачи для пользователя
func (s *TimeTrackingService) BeginTaskForUser(pasportSeries, pasportNumber string, taskId int32) error {
	const op = "TimeTrackingService: BeginTaskForUser"

	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Int("taskId", int(taskId)))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		return processStorageError(op, err, false)
	}

	Logger.Debug(op+": user found", slog.Int("user", int(user.Id)))

	// Поиск задачи по идентификатору
	filter := map[string]any{
		"id": taskId,
	}
	task, err := s.FindTasksByFilter(filter, 1, 0)
	if err != nil {
		return processStorageError(op, err, false)
	}

	Logger.Debug(op+": task found", slog.Int("task", int(task[0].Id)))

	if task[0].WorkFrom != (time.Time{}) {
		return processStorageError(op, errors.New("task already started"), true)
	}

	// Начало задачи
	updateData := map[string]any{
		"work_from": time.Now().UTC(),
		"user_id":   user.Id,
	}
	err = s.storage.Update(TaskCollection, filter, updateData)
	if err != nil {
		return processStorageError(op, err, true)
	}

	Logger.Debug(op+": task started", slog.Int("userId", int(user.Id)), slog.Int("task", int(task[0].Id)))
	return nil
}

// Завершение задачи для пользователя
func (s *TimeTrackingService) EndTaskForUser(pasportSeries, pasportNumber string, taskId int32) error {
	const op = "TimeTrackingService: EndTaskForUser"

	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Int("taskId", int(taskId)))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		return processStorageError(op, err, false)
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser user found", slog.Int("user", int(user.Id)))

	// Поиск задачи по идентификатору
	filter := map[string]any{
		"id": taskId,
	}
	task, err := s.FindTasksByFilter(filter, 1, 0)
	if err != nil {
		return processStorageError(op, err, false)
	}

	if len(task) == 0 {
		return processStorageError(op, &NotFoundError{"task not found"}, true)
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser task found", slog.Int("task", int(task[0].Id)))

	if task[0].WorkFrom == (time.Time{}) {
		return processStorageError(op, errors.New("task not started"), true)
	}

	// Конец задачи
	updateData := map[string]any{
		"cost":      task[0].Cost + time.Now().UTC().Sub(task[0].WorkFrom),
		"work_from": nil,
	}
	err = s.storage.Update(TaskCollection, filter, updateData)
	if err != nil {
		return processStorageError(op, err, true)
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser task ended", slog.Int("userId", int(user.Id)), slog.Int("task", int(task[0].Id)))
	return nil
}

// Удаление пользователя
func (s *TimeTrackingService) DeleteUser(pasportSeries, pasportNumber string) error {
	const op = "TimeTrackingService: DeleteUser"

	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		return processStorageError(op, err, false)
	}

	Logger.Debug("TimeTrackingService: DeleteUser user found", slog.Int("user", int(user.Id)))

	// Удаление пользователя
	err = s.storage.Delete(UserCollection, user.Id)
	if err != nil {
		return processStorageError(op, err, true)
	}

	Logger.Debug("TimeTrackingService: DeleteUser user deleted", slog.Int("userId", int(user.Id)))
	return nil
}

// Обновление информации о пользователе
func (s *TimeTrackingService) UpdateInfoUser(pasportSeries, pasportNumber string, info map[string]any) error {
	const op = "TimeTrackingService: UpdateInfoUser"

	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Any("info", info))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil {
		return processStorageError(op, err, false)
	}

	Logger.Debug("TimeTrackingService: UpdateInfoUser user found", slog.Int("user", int(user.Id)))

	// Обновление информации о пользователе
	filter := map[string]any{
		"id": user.Id,
	}
	err = s.storage.Update(UserCollection, filter, info)
	if err != nil {
		return processStorageError(op, err, true)
	}

	Logger.Debug("TimeTrackingService: UpdateInfoUser user updated", slog.Int("userId", int(user.Id)))

	return nil
}

// Создание пользователя
func (s *TimeTrackingService) CreateUser(pasportSeries, pasportNumber string) (int32, error) {
	const op = "TimeTrackingService: CreateUser"
	Logger.Debug(op, slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Поиск пользователя по паспорту
	user, err := s.FindUserByPassport(pasportSeries, pasportNumber)
	if err != nil && !errors.Is(err, &NotFoundError{}) {
		return 0, processStorageError(op, err, false)
	}

	// Пользователь уже существует, возвращаем его идентификатор
	if user != nil {
		Logger.Debug("TimeTrackingService: CreateUser user found", slog.Int("user", int(user.Id)))
		return user.Id, nil
	}

	// Создание пользователя
	userData := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}

	newId, err := s.storage.Insert(UserCollection, userData)
	if err != nil {
		return 0, processStorageError(op, err, true)
	}

	Logger.Debug("TimeTrackingService: CreateUser user created", slog.Int("userId", int(newId)))
	return newId, nil
}
