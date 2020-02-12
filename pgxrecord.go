// Package pgxrecord is a tiny framework for CRUD operations and data mapping.
package pgxrecord

import (
	"context"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgsql"
	"github.com/jackc/pgx/v4"
)

type Queryer interface {
	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
}

type Op int8

const (
	unspecifiedOp Op = iota
	InsertOp
	UpdateOp
)

type BeforeSaver interface {
	// BeforeSave returns an error if the operation should be canceled. op is either InsertOp or UpdateOp.
	BeforeSave(op Op) error
}

type Inserter interface {
	InsertStatement() *pgsql.InsertStatement
}

type InsertScanner interface {
	// InsertScan scans the row generated by the returning clause into the record.
	InsertScan(pgx.Rows) error
}

type Updater interface {
	UpdateStatement() *pgsql.UpdateStatement
}

type Deleter interface {
	TableName() string
	WherePrimaryKey() *pgsql.SelectStatement
}

type Selector interface {
	// SelectStatement returns a select statement that selects a record.
	SelectStatement() *pgsql.SelectStatement

	// SelectScan scans the current row into the record.
	SelectScan(pgx.Rows) error
}

type SelectCollection interface {
	// NewRecord allocates and returns a new record that can be appended to this collection.
	NewRecord() Selector

	// Append appends record to the collection.
	Append(record Selector)
}

type PgErrorMapper interface {
	// MapPgError converts a *pgconn.PgError to another type of error. For example, a unique constraint violation may be
	// converted to an application specific validation error.
	MapPgError(*pgconn.PgError) error
}

func tryMapPgError(record interface{}, err error) error {
	if mapper, ok := record.(PgErrorMapper); ok {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			return mapper.MapPgError(pgErr)
		}
	}

	return err
}

type multipleRowsError struct {
	rowCount int64
}

func (e *multipleRowsError) Error() string {
	return fmt.Sprintf("expected 1 row got %d", e.rowCount)
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "not found"
}

// NotFound returns true if err is a not found error.
func NotFound(err error) bool {
	_, ok := err.(*notFoundError)
	return ok
}

// Insert inserts record into db. If record implements BeforeSaver then BeforeSave will be called. If an error is
// returned the Insert is aborted.
func Insert(ctx context.Context, db Queryer, record Inserter) error {
	if bs, ok := record.(BeforeSaver); ok {
		err := bs.BeforeSave(InsertOp)
		if err != nil {
			return err
		}
	}

	stmt := record.InsertStatement()
	sql, args := pgsql.Build(stmt)

	var f scanFunc
	if record, ok := record.(InsertScanner); ok {
		f = func(rows pgx.Rows) error { return record.InsertScan(rows) }
	}

	return queryOne(ctx, db, record, sql, args, f)
}

// Update updates record in db. If record implements BeforeSaver then BeforeSave will be called. If an error is
// returned the Update is aborted. If the update query does not affect exactly one record an error will be returned.
func Update(ctx context.Context, db Queryer, record Updater) error {
	if bs, ok := record.(BeforeSaver); ok {
		err := bs.BeforeSave(UpdateOp)
		if err != nil {
			return err
		}
	}

	stmt := record.UpdateStatement()
	sql, args := pgsql.Build(stmt)
	return queryOne(ctx, db, record, sql, args, nil)
}

// Delete deletes record in db. If the delete query does affect exactly one record an error will be returned.
func Delete(ctx context.Context, db Queryer, record Deleter) error {
	stmt := pgsql.Delete(record.TableName()).Apply(record.WherePrimaryKey())
	sql, args := pgsql.Build(stmt)
	return queryOne(ctx, db, record, sql, args, nil)
}

// SelectOne selects a single record from db into record. It applies scopes to the SQL statement. An error will be
// returned if no rows are found. Check for this case with the NotFound function. If multiple rows are selected an
// error will be returned.
func SelectOne(ctx context.Context, db Queryer, record Selector, scopes ...*pgsql.SelectStatement) error {
	stmt := record.SelectStatement().Apply(scopes...)
	sql, args := pgsql.Build(stmt)
	return queryOne(ctx, db, record, sql, args, record.SelectScan)
}

type scanFunc func(rows pgx.Rows) error

func queryOne(ctx context.Context, db Queryer, record interface{}, sql string, queryArgs []interface{}, scan scanFunc) error {
	rows, err := db.Query(ctx, sql, queryArgs...)
	if err != nil {
		return err
	}

	if rows.Next() && scan != nil {
		err = scan(rows)
		if err != nil {
			rows.Close()
			return tryMapPgError(record, err)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return tryMapPgError(record, rows.Err())
	}

	rowsAffected := rows.CommandTag().RowsAffected()
	if rowsAffected == 0 {
		return &notFoundError{}
	}
	if rowsAffected > 1 {
		return &multipleRowsError{rowCount: rowsAffected}
	}

	return nil
}

// SelectAll selects records from db into collection. It applies scopes to the SQL statement.
func SelectAll(ctx context.Context, db Queryer, collection SelectCollection, scopes ...*pgsql.SelectStatement) error {
	record := collection.NewRecord()
	stmt := record.SelectStatement().Apply(scopes...)
	sql, args := pgsql.Build(stmt)

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return tryMapPgError(record, err)
	}

	rowCount := 0
	for rows.Next() {
		if rowCount > 0 {
			record = collection.NewRecord()
		}
		err := record.SelectScan(rows)
		if err != nil {
			return err
		}

		collection.Append(record)
		rowCount++
	}
	if rows.Err() != nil {
		return tryMapPgError(record, rows.Err())
	}

	return nil
}
