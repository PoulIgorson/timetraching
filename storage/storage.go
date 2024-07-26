package storage

const UserCollection = "users"

const TaskCollection = "tasks"

type Record struct {
	Collection string
	Id         int32
	Fields     map[string]any
}

type RecordReader interface {
	Next() bool
	Read() (*Record, error)
}

type Storage interface {
	// Select - получить записи по фильтру с пагинацией
	Select(collection string, filter map[string]any, limit, offset int) (RecordReader, error)

	// Update - обновить запись
	Update(collection string, filter map[string]any, update map[string]any) error

	// Insert - добавить запись, возвращает идентификатор
	Insert(collection string, data map[string]any) (int32, error)

	// Delete - удалить запись по идентификатору
	Delete(collection string, id int32) error
}
