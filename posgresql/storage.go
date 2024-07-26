package posgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/doug-martin/goqu/v9"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"

	. "timetracking/storage"
)

var Logger = slog.Default()

var _ Storage = (*PosgresqlStorage)(nil)

type PsqlConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

func (config *PsqlConfig) ConnInfo() string {
	return "postgres://" + config.Username + ":" + config.Password + "@" + config.Host + ":" + fmt.Sprint(config.Port) + "/" + config.Database
}

type PosgresqlStorage struct {
	db *pgx.Conn
}

func migrating(pathMigrations string, connInfo string) error {
	m, err := migrate.New(
		pathMigrations,
		connInfo,
	)
	if err != nil {
		return fmt.Errorf("posgresql: migrate failed: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("posgresql: migrate failed: %w", err)
	}

	return nil
}

func NewPosgresqlStorage(config *PsqlConfig) (*PosgresqlStorage, error) {
	if config == nil {
		Logger.Info("posgresql: config is nil")
		return nil, errors.New("posgresql: config is nil")
	}

	Logger.Debug("posgresql: config", slog.String("config", fmt.Sprintf("%+v", config)))

	db, err := pgx.Connect(context.Background(), config.ConnInfo())
	if err != nil {
		Logger.Info("posgresql: connection failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("posgresql: connection failed: %w", err)
	}

	if err := migrating("file://migrations", config.ConnInfo()); err != nil {
		Logger.Info("posgresql: migrating failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("posgresql: migrating failed: %w", err)
	}

	Logger.Info("posgresql: connected")

	return &PosgresqlStorage{
		db: db,
	}, nil
}

func (s *PosgresqlStorage) Close() error {
	Logger.Info("posgresql: closing")
	return s.db.Close(context.Background())
}

type recordReader struct {
	rows pgx.Rows
}

func (r *recordReader) Next() bool {
	return r.rows.Next()
}

func (r *recordReader) Read() (*Record, error) {
	if r.rows == nil {
		return nil, sql.ErrNoRows
	}
	columns := r.rows.FieldDescriptions()

	rowData, err := r.rows.Values()
	if err != nil {
		Logger.Info("posgresql: read failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("posgresql: read failed: %w", err)
	}

	rowMap := map[string]any{}
	for i, column := range columns {
		rowMap[column.Name] = rowData[i]
	}

	return &Record{
		Id:     rowMap["id"].(int32),
		Fields: rowMap,
	}, nil
}

func (s *PosgresqlStorage) Select(collection string, filter map[string]any, limit, offset int) (RecordReader, error) {
	Logger.Debug("posgresql: select", slog.String("collection", collection), slog.Any("filter", filter), slog.Int("limit", limit), slog.Int("offset", offset))

	exps := []goqu.Expression{}
	for k, v := range filter {
		exps = append(exps, goqu.I(k).Eq(v))
	}

	query, _, err := goqu.From(collection).Where(exps...).Limit(uint(limit)).Offset(uint(offset)).ToSQL()
	if err != nil {
		Logger.Info("posgresql: select failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("posgresql: select failed: %w", err)
	}

	Logger.Debug("posgresql: select", slog.String("query", query))

	rows, err := s.db.Query(context.Background(), query)
	if err != nil {
		Logger.Info("posgresql: select failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("posgresql: select failed: %w", err)
	}

	Logger.Debug("posgresql: select success")

	return &recordReader{rows: rows}, nil
}

func (s *PosgresqlStorage) Update(collection string, filter map[string]any, update map[string]any) error {
	Logger.Debug("posgresql: update", slog.String("collection", collection), slog.Any("filter", filter), slog.Any("update", update))

	exps := []goqu.Expression{}
	for k, v := range filter {
		exps = append(exps, goqu.I(k).Eq(v))
	}
	query, _, err := goqu.Update(collection).Set(update).Where(exps...).ToSQL()
	if err != nil {
		Logger.Info("posgresql: update failed", slog.String("error", err.Error()))
		return fmt.Errorf("posgresql: update failed: %w", err)
	}

	Logger.Debug("posgresql: update", slog.String("query", query))

	_, err = s.db.Exec(context.Background(), query)
	if err != nil {
		Logger.Info("posgresql: update failed", slog.String("error", err.Error()))
		return fmt.Errorf("posgresql: update failed: %w", err)
	}

	Logger.Debug("posgresql: update success")

	return nil
}

func (s *PosgresqlStorage) Insert(collection string, data map[string]any) (int32, error) {
	Logger.Debug("posgresql: insert", slog.String("collection", collection), slog.Any("data", data))

	query, _, err := goqu.Insert(collection).Rows(data).Returning(goqu.C("id")).ToSQL()
	if err != nil {
		Logger.Info("posgresql: insert failed", slog.String("error", err.Error()))
		return 0, fmt.Errorf("posgresql: insert failed: %w", err)
	}

	Logger.Debug("posgresql: insert", slog.String("query", query))

	var id int32
	err = s.db.QueryRow(context.Background(), query).Scan(&id)
	if err != nil {
		Logger.Info("posgresql: insert failed", slog.String("error", err.Error()))
		return 0, fmt.Errorf("posgresql: insert failed: %w", err)
	}

	Logger.Debug("posgresql: insert success")
	return id, nil
}

func (s *PosgresqlStorage) Delete(collection string, id int32) error {
	Logger.Debug("posgresql: delete", slog.String("collection", collection), slog.Int("id", int(id)))

	query, _, err := goqu.Delete(collection).Where(goqu.C("id").Eq(id)).ToSQL()
	if err != nil {
		Logger.Info("posgresql: delete failed", slog.String("error", err.Error()))
		return fmt.Errorf("posgresql: delete failed: %w", err)
	}

	Logger.Debug("posgresql: delete", slog.String("query", query))

	_, err = s.db.Exec(context.Background(), query)
	if err != nil {
		Logger.Info("posgresql: delete failed", slog.String("error", err.Error()))
		return fmt.Errorf("posgresql: delete failed: %w", err)
	}

	Logger.Debug("posgresql: delete success")

	return nil
}
