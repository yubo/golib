package orm

import (
	"context"
	"database/sql"
)

type DataType string

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
	RawDB() *sql.DB
	Close() error
	Begin() (Tx, error)
	BeginTx(ctx context.Context, ops *sql.TxOptions) (Tx, error)
	ExecRows(bytes []byte) error // like mysql < a.sql

	Interface
}

type Tx interface {
	Tx() *sql.Tx
	Rollback() error
	Commit() error

	Interface
}

type rawDB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type Interface interface {
	Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecLastId(sql string, args ...interface{}) (int64, error)
	ExecNum(sql string, args ...interface{}) (int64, error)
	ExecNumErr(s string, args ...interface{}) error
	Query(query string, args ...interface{}) *Rows

	Insert(sample interface{}, opts ...SqlOption) error
	InsertLastId(sample interface{}, opts ...SqlOption) (int64, error)
	List(into interface{}, opts ...SqlOption) error
	Get(into interface{}, opts ...SqlOption) error
	Update(sample interface{}, opts ...SqlOption) error
	Delete(sample interface{}, opts ...SqlOption) error
}

type Driver interface {
	// refer: https://gorm.io/docs/migration.html
	AutoMigrate(sample interface{}, opts ...SqlOption) error

	//  parse datatype
	ParseField(opts *field)

	// Database
	CurrentDatabase() string
	FullDataTypeOf(field *FieldOptions) string

	// Tables
	CreateTable(o *SqlOptions) error
	DropTable(o *SqlOptions) error
	HasTable(tableName string) bool
	GetTables() (tableList []string, err error)

	// Columns
	AddColumn(field string, o *SqlOptions) error
	DropColumn(field string, o *SqlOptions) error
	AlterColumn(field string, o *SqlOptions) error
	MigrateColumn(expect, actual *FieldOptions, o *SqlOptions) error
	HasColumn(field string, o *SqlOptions) bool
	ColumnTypes(o *SqlOptions) ([]FieldOptions, error)

	// Indexes
	CreateIndex(name string, o *SqlOptions) error
	DropIndex(name string, o *SqlOptions) error
	HasIndex(name string, o *SqlOptions) bool
}

type nonDriver struct{}

func (b nonDriver) AutoMigrate(sample interface{}, opts ...SqlOption) error         { return nil }
func (b nonDriver) ParseField(opts *field)                                          {}
func (b nonDriver) CurrentDatabase() string                                         { return "" }
func (b nonDriver) FullDataTypeOf(field *FieldOptions) string                       { return "" }
func (b nonDriver) CreateTable(o *SqlOptions) error                                 { return nil }
func (b nonDriver) DropTable(o *SqlOptions) error                                   { return nil }
func (b nonDriver) HasTable(tableName string) bool                                  { return false }
func (b nonDriver) GetTables() (tableList []string, err error)                      { return nil, nil }
func (b nonDriver) AddColumn(field string, o *SqlOptions) error                     { return nil }
func (b nonDriver) DropColumn(field string, o *SqlOptions) error                    { return nil }
func (b nonDriver) AlterColumn(field string, o *SqlOptions) error                   { return nil }
func (b nonDriver) MigrateColumn(expect, actual *FieldOptions, o *SqlOptions) error { return nil }
func (b nonDriver) HasColumn(field string, o *SqlOptions) bool                      { return false }
func (b nonDriver) ColumnTypes(o *SqlOptions) ([]FieldOptions, error)               { return nil, nil }
func (b nonDriver) CreateIndex(name string, o *SqlOptions) error                    { return nil }
func (b nonDriver) DropIndex(name string, o *SqlOptions) error                      { return nil }
func (b nonDriver) HasIndex(name string, o *SqlOptions) bool                        { return false }
