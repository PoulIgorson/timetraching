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

func get[T any](fields map[string]any, name string) T {
	if v, ok := fields[name].(T); ok {
		return v
	}
	return *new(T)
}

// Методы

// Находит пользователей по фильтру с пагинацией, возвращает список пользователей
// Если не находит записей возвращает ErrNoRows
func (s *TimeTrackingService) FindUsersByFilter(filter map[string]any, limit, offset int) ([]*User, error) {
	Logger.Debug("TimeTrackingService: FindUsersByFilter", slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: FindUsersByFilter", slog.String("error", "storage is nil"))
		return nil, ErrInternal
	}

	// Получение пользователей по фильтру с пагинацией
	reader, err := s.storage.Select(UserCollection, filter, limit, offset)
	if err != nil {
		// Перехват отсутствия записей
		if errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: FindUsersByFilter", slog.String("info", "no users found"))
			return nil, sql.ErrNoRows
		}

		// Иначе возвращаем ошибку
		Logger.Info("TimeTrackingService: FindUsersByFilter failed", slog.String("error", err.Error()))
		return nil, errors.Join(ErrStorage, err)
	}

	// Чтение пользователей
	var users []*User
	for reader.Next() {
		record, err := reader.Read()
		if err != nil && !errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: FindUsersByFilter failed", slog.String("error", err.Error()))
			return nil, errors.Join(ErrStorage, err)
		}
		user := &User{
			Id:            get[int32](record.Fields, "id"),
			PasportSeries: get[string](record.Fields, "pasport_series"),
			PasportNumber: get[string](record.Fields, "pasport_number"),
			Surname:       get[string](record.Fields, "surname"),
			Name:          get[string](record.Fields, "name"),
			Patronymic:    get[string](record.Fields, "patronymic"),
			Address:       get[string](record.Fields, "address"),
		}
		users = append(users, user)
	}

	Logger.Debug("TimeTrackingService: FindUsersByFilter users found", slog.Int("count", len(users)))
	return users, nil
}

// Находит задач по фильтру с пагинацией, возвращает список задач
// Если не находит записей возвращает ErrNoRows
func (s *TimeTrackingService) FindTasksByFilter(filter map[string]any, limit, offset int) ([]*Task, error) {
	Logger.Debug("TimeTrackingService: FindTasksByFilter", slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: FindTasksByFilter", slog.String("error", "storage is nil"))
		return nil, ErrInternal
	}

	// Получение задач по фильтру с пагинацией
	reader, err := s.storage.Select(TaskCollection, filter, limit, offset)
	if err != nil {
		// Перехват отсутствия записей
		if errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: FindTasksByFilter", slog.String("info", "no tasks found"))
			return nil, sql.ErrNoRows
		}

		// Иначе возвращаем ошибку
		Logger.Info("TimeTrackingService: FindTasksByFilter failed", slog.String("error", err.Error()))
		return nil, errors.Join(ErrStorage, err)
	}

	// Чтение задач
	var tasks []*Task
	for reader.Next() {
		record, err := reader.Read()
		if err != nil && !errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: FindTasksByFilter failed", slog.String("error", err.Error()))
			return nil, errors.Join(ErrStorage, err)
		}
		task := &Task{
			Id:          get[int32](record.Fields, "id"),
			Title:       get[string](record.Fields, "title"),
			Description: get[string](record.Fields, "description"),
			PeriodFrom:  get[time.Time](record.Fields, "period_from"),
			PeriodTo:    get[time.Time](record.Fields, "period_to"),

			UserId:   get[int32](record.Fields, "user_id"),
			Cost:     time.Duration(get[int64](record.Fields, "cost")),
			WorkFrom: get[time.Time](record.Fields, "work_from"),
		}
		tasks = append(tasks, task)
	}

	Logger.Debug("TimeTrackingService: FindTasksByFilter tasks found", slog.Int("count", len(tasks)))
	return tasks, nil
}

// Вычисляет стоимость задачи по идентификатору пользователя
func (s *TimeTrackingService) CalculateCostByUser(pasportSeries, pasportNumber string, begin, end time.Time) ([]string, error) {
	Logger.Debug("TimeTrackingService: CalculateCostByUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: CalculateCostByUser", slog.String("error", "storage is nil"))
		return nil, ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: CalculateCostByUser failed", slog.String("error", err.Error()))
		return nil, err
	}

	Logger.Debug("TimeTrackingService: CalculateCostByUser user found", slog.Int("user", int(user[0].Id)))

	// Получение задач пользователя
	filter = map[string]any{
		"user_id": user[0].Id,
	}
	reader, err := s.storage.Select(TaskCollection, filter, 0, 0)
	if err != nil {
		// Перехват отсутствия записей
		if errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: CalculateCostByUser", slog.String("error", "no tasks found"))
			return []string{}, nil
		}

		// Иначе возвращаем ошибку
		Logger.Info("TimeTrackingService: CalculateCostByUser failed", slog.String("error", err.Error()))
		return nil, errors.Join(ErrStorage, err)
	}

	// Подсчет затраченного времени
	var costs []string
	for reader.Next() {
		record, err := reader.Read()
		if err != nil && !errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: CalculateCostByUser failed", slog.String("error", err.Error()))
			return nil, errors.Join(ErrStorage, err)
		}
		periodFrom := get[time.Time](record.Fields, "period_from")
		periodTo := get[time.Time](record.Fields, "period_to")

		if periodTo.Before(begin) || periodFrom.After(end) {
			continue
		}

		fmt.Printf("%t", record.Fields)

		costs = append(costs, fmt.Sprintf("%d-%v", record.Id, time.Duration(get[int64](record.Fields, "cost")).Truncate(time.Second)))
	}

	sort.Slice(costs, func(i, j int) bool {
		costI := strings.Split(costs[i], "-")
		costJ := strings.Split(costs[j], "-")
		return costI[1] > costJ[1]
	})

	Logger.Debug("TimeTrackingService: CalculateCostByUser cost calculated", slog.Int("userId", int(user[0].Id)), slog.Any("costs", costs))
	return costs, nil
}

// Запуск задачи для пользователя
func (s *TimeTrackingService) BeginTaskForUser(pasportSeries, pasportNumber string, taskId int32) error {
	Logger.Debug("TimeTrackingService: BeginTaskForUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Int("taskId", int(taskId)))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: BeginTaskForUser", slog.String("error", "storage is nil"))
		return ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: BeginTaskForUser failed", slog.String("error", err.Error()))
		return err
	}

	Logger.Debug("TimeTrackingService: BeginTaskForUser user found", slog.Int("user", int(user[0].Id)))

	// Поиск задачи по идентификатору
	filter = map[string]any{
		"id": taskId,
	}
	task, err := s.FindTasksByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: BeginTaskForUser failed", slog.String("error", err.Error()))
		return err
	}

	Logger.Debug("TimeTrackingService: BeginTaskForUser task found", slog.Int("task", int(task[0].Id)))

	if task[0].WorkFrom != (time.Time{}) {
		Logger.Info("TimeTrackingService: BeginTaskForUser failed", slog.String("error", "task already started"))
		return fmt.Errorf("task already started")
	}

	// Начало задачи
	updateData := map[string]any{
		"work_from": time.Now().UTC(),
		"user_id":   user[0].Id,
	}
	err = s.storage.Update(TaskCollection, filter, updateData)
	if err != nil {
		Logger.Info("TimeTrackingService: BeginTaskForUser failed", slog.String("error", err.Error()))
		return errors.Join(ErrStorage, err)
	}

	Logger.Debug("TimeTrackingService: BeginTaskForUser task started", slog.Int("userId", int(user[0].Id)), slog.Int("task", int(task[0].Id)))

	return nil
}

// Завершение задачи для пользователя
func (s *TimeTrackingService) EndTaskForUser(pasportSeries, pasportNumber string, taskId int32) error {
	Logger.Debug("TimeTrackingService: EndTaskForUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Int("taskId", int(taskId)))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: EndTaskForUser", slog.String("error", "storage is nil"))
		return ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: EndTaskForUser failed", slog.String("error", err.Error()))
		return err
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser user found", slog.Int("user", int(user[0].Id)))

	// Поиск задачи по идентификатору
	filter = map[string]any{
		"id": taskId,
	}
	task, err := s.FindTasksByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: EndTaskForUser failed", slog.String("error", err.Error()))
		return err
	}

	if len(task) == 0 {
		Logger.Info("TimeTrackingService: EndTaskForUser failed", slog.String("error", "task not found"))
		return fmt.Errorf("task not found")
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser task found", slog.Int("task", int(task[0].Id)))

	if task[0].WorkFrom == (time.Time{}) {
		Logger.Info("TimeTrackingService: EndTaskForUser failed", slog.String("error", "task not started"))
		return fmt.Errorf("task not started")
	}

	// Конец задачи
	updateData := map[string]any{
		"cost":      task[0].Cost + time.Now().UTC().Sub(task[0].WorkFrom),
		"work_from": nil,
	}
	err = s.storage.Update(TaskCollection, filter, updateData)
	if err != nil {
		Logger.Info("TimeTrackingService: EndTaskForUser failed", slog.String("error", err.Error()))
		return errors.Join(ErrStorage, err)
	}

	Logger.Debug("TimeTrackingService: EndTaskForUser task ended", slog.Int("userId", int(user[0].Id)), slog.Int("task", int(task[0].Id)))

	return nil
}

// Удаление пользователя
func (s *TimeTrackingService) DeleteUser(pasportSeries, pasportNumber string) error {
	Logger.Debug("TimeTrackingService: DeleteUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: DeleteUser", slog.String("error", "storage is nil"))
		return ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: DeleteUser failed", slog.String("error", err.Error()))
		return err
	}

	Logger.Debug("TimeTrackingService: DeleteUser user found", slog.Int("user", int(user[0].Id)))

	// Удаление пользователя
	err = s.storage.Delete(UserCollection, user[0].Id)
	if err != nil {
		Logger.Info("TimeTrackingService: DeleteUser failed", slog.String("error", err.Error()))
		return errors.Join(ErrStorage, err)
	}

	Logger.Debug("TimeTrackingService: DeleteUser user deleted", slog.Int("userId", int(user[0].Id)))

	return nil
}

// Обновление информации о пользователе
func (s *TimeTrackingService) UpdateInfoUser(pasportSeries, pasportNumber string, info map[string]any) error {
	Logger.Debug("TimeTrackingService: UpdateInfoUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber), slog.Any("info", info))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: UpdateInfoUser", slog.String("error", "storage is nil"))
		return ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		Logger.Info("TimeTrackingService: UpdateInfoUser failed", slog.String("error", err.Error()))
		return err
	}

	Logger.Debug("TimeTrackingService: UpdateInfoUser user found", slog.Int("user", int(user[0].Id)))

	// Обновление информации о пользователе
	err = s.storage.Update(UserCollection, filter, info)
	if err != nil {
		Logger.Info("TimeTrackingService: UpdateInfoUser failed", slog.String("error", err.Error()))
		return errors.Join(ErrStorage, err)
	}

	Logger.Debug("TimeTrackingService: UpdateInfoUser user updated", slog.Int("userId", int(user[0].Id)))

	return nil
}

// Создание пользователя
func (s *TimeTrackingService) CreateUser(pasportSeries, pasportNumber string) (int32, error) {
	Logger.Debug("TimeTrackingService: CreateUser", slog.String("pasportSeries", pasportSeries), slog.String("pasportNumber", pasportNumber))

	// Проверка существования хранилища
	if s.storage == nil {
		Logger.Info("TimeTrackingService: CreateUser", slog.String("error", "storage is nil"))
		return 0, ErrInternal
	}

	// Поиск пользователя по паспорту
	filter := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}
	user, err := s.FindUsersByFilter(filter, 1, 0)
	if err != nil {
		if !errors.As(err, sql.ErrNoRows) {
			Logger.Info("TimeTrackingService: CreateUser failed", slog.String("error", err.Error()))
			return 0, sql.ErrNoRows
		}
	}

	// Пользователь уже существует, возвращаем его идентификатор
	if len(user) > 0 {
		Logger.Debug("TimeTrackingService: CreateUser user found", slog.Int("user", int(user[0].Id)))
		return user[0].Id, nil
	}

	// Создание пользователя
	userData := map[string]any{
		"pasport_series": pasportSeries,
		"pasport_number": pasportNumber,
	}

	newId, err := s.storage.Insert(UserCollection, userData)
	if err != nil {
		Logger.Info("TimeTrackingService: CreateUser failed", slog.String("error", err.Error()))
		return 0, errors.Join(ErrStorage, err)
	}

	Logger.Debug("TimeTrackingService: CreateUser user created", slog.Int("userId", int(newId)))

	return newId, nil
}
