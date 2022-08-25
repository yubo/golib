package orm

import (
	"context"
	"database/sql"
)

type DataType string

type DBFactory func(db Execer, opts *DBOptions) Driver

const (
	Bool   DataType = "bool"
	Int    DataType = "int"
	Uint   DataType = "uint"
	Float  DataType = "float"
	String DataType = "string"
	Time   DataType = "time"
	Bytes  DataType = "bytes"
)

type TimeType int64

const (
	UnixTime        TimeType = 1
	UnixSecond      TimeType = 2
	UnixMillisecond TimeType = 3
	UnixNanosecond  TimeType = 4
)

type DB interface {
	SqlDB() *sql.DB
	Close() error
	Begin() (Tx, error)
	BeginTx(ctx context.Context, ops *sql.TxOptions) (Tx, error)
	ExecRows(bytes []byte) error // like mysql < a.sql

	Interface
}

type RawDB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

type Tx interface {
	Tx() *sql.Tx
	Rollback() error
	Commit() error

	Interface
}

type Interface interface {
	Driver // ddl
	Store  // dml
	Execer
}

type Execer interface {
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	ExecLastId(ctx context.Context, sql string, args ...interface{}) (int64, error)
	ExecNum(ctx context.Context, sql string, args ...interface{}) (int64, error)
	ExecNumErr(ctx context.Context, s string, args ...interface{}) error
	Query(ctx context.Context, query string, args ...interface{}) *Rows
	WithRawDB(raw RawDB) Interface
	RawDB() RawDB
}

// DML
type Store interface {
	Insert(ctx context.Context, sample interface{}, opts ...Option) error
	InsertLastId(ctx context.Context, sample interface{}, opts ...Option) (int64, error)
	Get(ctx context.Context, into interface{}, opts ...Option) error
	List(ctx context.Context, into interface{}, opts ...Option) error
	Update(ctx context.Context, sample interface{}, opts ...Option) error
	Delete(ctx context.Context, sample interface{}, opts ...Option) error
}

// DDL
type Driver interface {
	// refer: https://gorm.io/docs/migration.html
	AutoMigrate(ctx context.Context, sample interface{}, opts ...Option) error

	//  parse datatype
	ParseField(opts *StructField)

	// Database
	CurrentDatabase(ctx context.Context) string
	FullDataTypeOf(field *StructField) string

	// Tables
	CreateTable(ctx context.Context, o *Options) error
	DropTable(ctx context.Context, o *Options) error
	HasTable(ctx context.Context, tableName string) bool
	GetTables(ctx context.Context) (tableList []string, err error)

	// Columns
	AddColumn(ctx context.Context, field string, o *Options) error
	DropColumn(ctx context.Context, field string, o *Options) error
	AlterColumn(ctx context.Context, field string, o *Options) error
	MigrateColumn(ctx context.Context, expect, actual *StructField, o *Options) error
	HasColumn(ctx context.Context, field string, o *Options) bool
	ColumnTypes(ctx context.Context, o *Options) ([]StructField, error)

	// Indexes
	CreateIndex(ctx context.Context, name string, o *Options) error
	DropIndex(ctx context.Context, name string, o *Options) error
	HasIndex(ctx context.Context, name string, o *Options) bool
}

type nonDriver struct{}

func (b nonDriver) AutoMigrate(ctx context.Context, sample interface{}, opts ...Option) error {
	return nil
}
func (b nonDriver) ParseField(opts *StructField)                                    {}
func (b nonDriver) CurrentDatabase(ctx context.Context) string                      { return "" }
func (b nonDriver) FullDataTypeOf(field *StructField) string                        { return "" }
func (b nonDriver) CreateTable(ctx context.Context, o *Options) error               { return nil }
func (b nonDriver) DropTable(ctx context.Context, o *Options) error                 { return nil }
func (b nonDriver) HasTable(ctx context.Context, tableName string) bool             { return false }
func (b nonDriver) GetTables(ctx context.Context) (tableList []string, err error)   { return nil, nil }
func (b nonDriver) AddColumn(ctx context.Context, field string, o *Options) error   { return nil }
func (b nonDriver) DropColumn(ctx context.Context, field string, o *Options) error  { return nil }
func (b nonDriver) AlterColumn(ctx context.Context, field string, o *Options) error { return nil }
func (b nonDriver) MigrateColumn(ctx context.Context, expect, actual *StructField, o *Options) error {
	return nil
}
func (b nonDriver) HasColumn(ctx context.Context, field string, o *Options) bool { return false }
func (b nonDriver) ColumnTypes(ctx context.Context, o *Options) ([]StructField, error) {
	return nil, nil
}
func (b nonDriver) CreateIndex(ctx context.Context, name string, o *Options) error { return nil }
func (b nonDriver) DropIndex(ctx context.Context, name string, o *Options) error   { return nil }
func (b nonDriver) HasIndex(ctx context.Context, name string, o *Options) bool     { return false }
