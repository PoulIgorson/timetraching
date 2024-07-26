package timetracking

// Внутренняя ошибка
type InternalError struct {
	msg string
}

type StorageError struct {
	msg string
}

type InvalidError struct {
	msg string
}

type NotFoundError struct {
	msg string
}

func (e InternalError) Error() string {
	if e.msg == "" {
		e.msg = "internal error"
	}
	return "timetracking: " + e.msg
}

func (e StorageError) Error() string {
	if e.msg == "" {
		e.msg = "storage error"
	}
	return "timetracking: " + e.msg
}

func (e InvalidError) Error() string {
	if e.msg == "" {
		e.msg = "invalid error"
	}
	return "timetracking: " + e.msg
}

func (e NotFoundError) Error() string {
	if e.msg == "" {
		e.msg = "not found"
	}
	return "timetracking: " + e.msg
}
