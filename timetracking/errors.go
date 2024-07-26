package timetracking

import (
	"errors"
)

// Внутренняя ошибка
var ErrInternal = &InternalError{"timetracking: internal server error"}

// Ошибка хранилища
var ErrStorage = errors.New("timetracking: storage error")

// Ошибка отсутствия пользователя
var ErrUserNotFound = errors.New("timetracking: user not found")

// Ошибка отсутствия задачи
var ErrTaskNotFound = errors.New("timetracking: task not found")

type InternalError struct {
	msg string
}

func (e *InternalError) Error() string {
	return e.msg
}
