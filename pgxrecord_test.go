package pgxrecord_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxtest"
	"github.com/jackc/pgxrecord"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultConnTestRunner pgxtest.ConnTestRunner

func init() {
	defaultConnTestRunner = pgxtest.DefaultConnTestRunner()
	defaultConnTestRunner.CreateConfig = func(ctx context.Context, t testing.TB) *pgx.ConnConfig {
		config, err := pgx.ParseConfig(os.Getenv("PGXRECORD_TEST_DATABASE"))
		require.NoError(t, err)
		return config
	}
}

func TestTableLoadAllColumns(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		require.Len(t, table.Columns, 3)
		expectedColumns := []pgxrecord.Column{
			{Name: "id", OID: pgtype.Int4OID, NotNull: true, PrimaryKey: true},
			{Name: "name", OID: pgtype.TextOID, NotNull: true, PrimaryKey: false},
			{Name: "age", OID: pgtype.Int4OID, NotNull: false, PrimaryKey: false},
		}
		for i := range expectedColumns {
			assert.Equalf(t, expectedColumns[i].Name, table.Columns[i].Name, "Column %d name", i+1)
			assert.Equalf(t, expectedColumns[i].OID, table.Columns[i].OID, "Column %d OID", i+1)
			assert.Equalf(t, expectedColumns[i].NotNull, table.Columns[i].NotNull, "Column %d not null", i+1)
			assert.Equalf(t, expectedColumns[i].PrimaryKey, table.Columns[i].PrimaryKey, "Column %d primary key", i+1)
		}
	})
}

func TestTableSelectQuery(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		require.Equal(t, `select "t"."id", "t"."name", "t"."age" from "t"`, table.SelectQuery())
	})
}

func TestTableNewRecord(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		record := table.NewRecord()
		require.Equal(t, map[string]any{"id": nil, "name": nil, "age": nil}, record.Attributes())

		record.SetAttributes(map[string]any{"name": "John", "age": 42})
		require.Equal(t, map[string]any{"id": nil, "name": "John", "age": 42}, record.Attributes())
	})
}

func TestTableFindByPK(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		var id int32
		err = tx.QueryRow(ctx, `insert into t (name, age) values ('John', 42) returning id`).Scan(&id)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		record, err := table.FindByPK(ctx, conn, id)
		require.NoError(t, err)
		require.Equal(t, map[string]any{"id": int32(1), "name": "John", "age": int32(42)}, record.Attributes())
	})
}

func TestRecordSetAndGet(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		record := table.NewRecord()

		err = record.Set("name", "John")
		require.NoError(t, err)

		name, err := record.Get("name")
		require.NoError(t, err)
		require.Equal(t, "John", name)
	})
}

func TestRecordSaveInsert(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		record := table.NewRecord()
		record.SetAttributes(map[string]any{"name": "John", "age": 42})
		err = record.Save(ctx, tx)
		require.NoError(t, err)

		require.Equal(t, map[string]any{"id": int32(1), "name": "John", "age": int32(42)}, record.Attributes())
	})
}

func TestRecordSaveUpdate(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create table t (
	id int primary key generated by default as identity,
	name text not null,
	age int
)`)
		require.NoError(t, err)

		var id int32
		err = tx.QueryRow(ctx, `insert into t (name, age) values ('John', 42) returning id`).Scan(&id)
		require.NoError(t, err)

		table := &pgxrecord.Table{
			Name: pgx.Identifier{"t"},
		}
		err = table.LoadAllColumns(ctx, tx)
		require.NoError(t, err)
		table.Finalize()

		record, err := table.FindByPK(ctx, conn, id)
		require.NoError(t, err)
		require.Equal(t, map[string]any{"id": int32(1), "name": "John", "age": int32(42)}, record.Attributes())

		record.MustSet("name", "Bill")
		err = record.Save(ctx, conn)
		require.NoError(t, err)

		record, err = table.FindByPK(ctx, conn, id)
		require.NoError(t, err)
		require.Equal(t, map[string]any{"id": int32(1), "name": "Bill", "age": int32(42)}, record.Attributes())
	})
}

func TestSelectRow(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		type Person struct {
			ID   int32
			Name string
			Age  int32
		}

		person, err := pgxrecord.SelectRow(ctx, conn, `select 1, 'John', 42`, nil, pgx.RowToAddrOfStructByPos[Person])
		require.NoError(t, err)
		require.EqualValues(t, 1, person.ID)
		require.Equal(t, "John", person.Name)
		require.EqualValues(t, 42, person.Age)
	})
}

func TestSelectRowNoRows(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		type Person struct {
			ID   int32
			Name string
			Age  int32
		}

		person, err := pgxrecord.SelectRow(ctx, conn, `select 1, 'John', 42 where false`, nil, pgx.RowToAddrOfStructByPos[Person])
		require.ErrorIs(t, err, pgx.ErrNoRows)
		require.Nil(t, person)
	})
}
