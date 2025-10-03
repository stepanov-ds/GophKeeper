package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stepanov-ds/GophKeeper/internal/server/config"
	"github.com/stepanov-ds/GophKeeper/internal/utils/contextKeys"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
)

var (
	pool *pgxpool.Pool
)

func InitConnection() {
	config, err := pgxpool.ParseConfig(*config.DatabaseDSN)
	if err != nil {
		panic(err)
	}
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Error while init DB connection: %v\n", err)
	}
}

func RunMigrations() {
	goose.SetDialect("pgx")
	db, err := sql.Open("pgx", *config.DatabaseDSN)
	if err != nil {
		log.Fatalf("goose: failed to open DB connection: %v\n", err)
	}
	defer db.Close()

	migrationsDir := "migrations"

	if err := goose.Up(db, migrationsDir); err != nil {
		log.Fatalf("goose: failed to apply migrations: %v\n", err)
	}
}

func RegisterUser(mail string) error {
	query :=
		`
	INSERT INTO public.users("username")
	VALUES ($1);
	`

	_, err := pool.Exec(context.Background(), query, mail)

	return err
}
func CheckUser(mail string) error {
	query :=
		`
	SELECT username FROM public.users
	WHERE username = $1;
	`

	row := pool.QueryRow(context.Background(), query, mail)

	var a interface{}
	err := row.Scan(a)

	return err
}

func AddSecureData(ctx context.Context, username string, data string, metadata string) (int64, int64, error) {
	ctx, err := BeginTransaction(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("error while begin transaction: %w", err)
	}
	defer RollbackTransaction(ctx)

	query :=
		`
	INSERT INTO public.secure_data("user_id", "data", "metadata", "history_id", "is_active")
	SELECT 
		id as user_id,
    	$2 AS data,
    	$3 AS metadata,
    	-1 AS history_id,
		true AS is_active
	FROM users
	where username = $1
	RETURNING id;
	`

	row := pool.QueryRow(context.Background(), query, username, data, metadata)

	var secureDataID int64
	err = row.Scan(&secureDataID)

	if err != nil {
		return 0, 0, err
	}

	historyID, err := UpdateHistory(ctx, secureDataID, username, "ADD")

	if err != nil {
		return 0, 0, err
	}

	err = CommitTransaction(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("error while commit transaction: %w", err)
	}

	return secureDataID, historyID, err
}

func DeleteSecureData(ctx context.Context, id int64, username string) (int64, error) {
	ctx, err := BeginTransaction(ctx)
	if err != nil {
		return 0, fmt.Errorf("error while begin transaction: %w", err)
	}
	defer RollbackTransaction(ctx)
	query :=
	
	`
	UPDATE public.secure_data
	SET is_active = false
	WHERE id = $1 AND user_id = (SELECT id FROM public.users WHERE username = $2);
	`

	_, err = pool.Exec(ctx, query, id, username)

	if err != nil {
		return 0, err
	}

	historyID, err := UpdateHistory(ctx, id, username, "DELETE")

	if err != nil {
		return 0, err
	}

	err = CommitTransaction(ctx)
	if err != nil {
		return 0, fmt.Errorf("error while commit transaction: %w", err)
	}

	return historyID, err
}

func UpdateSecureData(ctx context.Context, id int64, username string, data string, metadata string) (int64, error) {
	ctx, err := BeginTransaction(ctx)
	if err != nil {
		return 0, fmt.Errorf("error while begin transaction: %w", err)
	}
	defer RollbackTransaction(ctx)
	query :=
	`
	UPDATE public.secure_data
	SET data = $3, metadata = $4
	WHERE id = $1 AND user_id = (SELECT id FROM public.users WHERE username = $2);
	`

	_, err = pool.Exec(ctx, query, id, username, data, metadata)

	if err != nil {
		return 0, err
	}

	historyID, err := UpdateHistory(ctx, id, username, "UPDATE")

	if err != nil {
		return 0, err
	}

	err = CommitTransaction(ctx)
	if err != nil {
		return 0, fmt.Errorf("error while commit transaction: %w", err)
	}

	return historyID, err
}


func SelectUpdatedSecureData(lastID int64, username string, limit int) ([]structs.SecureData, error) {
	query := 
	`
	SELECT id, data, metadata, is_active, history_id
	FROM public.secure_data
	WHERE history_id > $1 AND user_id = (SELECT id FROM users WHERE username = $2)
	ORDER BY id
	LIMIT $3;
	`

	rows, err := pool.Query(context.Background(), query, lastID, username, limit)

	if err != nil {
		rows.Close()
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[structs.SecureData])
}

func UpdateHistory(ctx context.Context, id int64, username string, method string) (int64, error) {
	query := 
	`
	INSERT INTO public.history("user_id", "secure_data_id", "method")
	SELECT 
		id as user_id,
    	$1 AS secure_data_id,
    	$3 AS method
	FROM users
	where username = $2
	RETURNING id;
	`

	row := pool.QueryRow(ctx, query, id, username, method)

	var historyID int64
	err := row.Scan(&historyID)

	if err != nil {
		return 0, err
	}

	query =
	`
	UPDATE public.secure_data
	SET history_id = $3
	WHERE id = $1 AND user_id = (SELECT id FROM public.users WHERE username = $2);
	`
	_, err = pool.Exec(ctx, query, id, username, historyID)

	return historyID, err
}

func BeginTransaction(ctx context.Context) (context.Context, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, contextKeys.Transaction, tx)
	return ctx, nil
}

func CommitTransaction(ctx context.Context) error {
	tx, ok := ctx.Value(contextKeys.Transaction).(pgx.Tx)
	if !ok {
		return fmt.Errorf("no transaction found in context")
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func RollbackTransaction(ctx context.Context) error {
	tx, ok := ctx.Value(contextKeys.Transaction).(pgx.Tx)
	if !ok {
		return fmt.Errorf("no transaction found in context")
	}
	if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
		return err
	}
	return nil
}
