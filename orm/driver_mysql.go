package orm

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

var (
	_                        Driver = &mysql{}
	DefaultMysqlTableOptions        = []QueryOption{
		WithTableOptions("ENGINE=InnoDB DEFAULT CHARACTER SET=utf8 COLLATE=utf8_unicode_ci"),
	}
)

func RegisterMysql() {
	Register("mysql", func(db Execer, opts *DBOptions) Driver {
		return &mysql{db, opts}
	})
}

type mysqlColumn struct {
	ColumnName             string
	IsNullable             sql.NullString
	Datatype               string
	CharacterMaximumLength sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
}

func (p *mysqlColumn) FiledOptions() StructField {
	ret := StructField{
		Name:           p.ColumnName,
		DriverDataType: p.Datatype,
	}

	if p.CharacterMaximumLength.Valid {
		ret.Size = util.Int64(p.CharacterMaximumLength.Int64)
	}

	if p.IsNullable.Valid {
		ret.NotNull = util.Bool(p.IsNullable.String != "YES")
	}

	return ret
}

// mysql m struct
type mysql struct {
	Execer
	*DBOptions
}

func (p *mysql) ParseField(f *StructField) {
	f.DriverDataType = p.driverDataTypeOf(f)
}

func (p *mysql) driverDataTypeOf(f *StructField) string {
	switch f.DataType {
	case Bool:
		return "boolean"
	case Int, Uint:
		return p.getSchemaIntAndUnitType(f)
	case Float:
		return p.getSchemaFloatType(f)
	case String:
		return p.getSchemaStringType(f)
	case Time:
		return p.getSchemaTimeType(f)
	case Bytes:
		return p.getSchemaBytesType(f)
	}

	return string(f.DataType)
}

func (p *mysql) getSchemaIntAndUnitType(f *StructField) string {
	sqlType := "bigint"

	switch size := util.Int64Value(f.Size); {
	case size <= 8:
		sqlType = "tinyint"
	case size <= 16:
		sqlType = "smallint"
	case size <= 24:
		sqlType = "mediumint"
	case size <= 32:
		sqlType = "int"
	}

	if f.DataType == Uint {
		sqlType += " unsigned"
	}

	if f.AutoIncrement {
		sqlType += " AUTO_INCREMENT"
	}

	return sqlType
}

func (p *mysql) getSchemaFloatType(f *StructField) string {
	size := util.Int64Value(f.Size)

	if size <= 32 {
		return "float"
	}

	return "double"
}

func (p *mysql) getSchemaStringType(f *StructField) string {
	size := util.Int64Value(f.Size)

	if size == 0 {
		if p.stringSize > 0 {
			size = int64(p.stringSize)
		} else {
			hasIndex := f.Has("index") || f.Has("unique") || f.Has("unique_index")
			// TEXT, GEOMETRY or JSON column can't have a default value
			if f.PrimaryKey || f.HasDefaultValue || hasIndex {
				size = 191 // utf8mb4
			}
		}
	}

	// 0 ~ 2^16  varchar
	if size > 0 && size < 16384 {
		f.Size = util.Int64(size)
		return fmt.Sprintf("varchar(%d)", size)
	}

	f.Size = nil
	if size >= 16384 && size < 65536 {
		return "text"
	}

	if size >= 65536 && size < int64(math.Pow(2, 24)) {
		return "mediumtext"
	}

	return "longtext"
}

func (p mysql) getSchemaTimeType(field *StructField) string {
	precision := ""

	if field.Precision != nil && *field.Precision > 0 {
		precision = fmt.Sprintf("(%d)", *field.Precision)
	}

	if util.BoolValue(field.NotNull) || field.PrimaryKey {
		return "datetime" + precision
	}
	return "datetime" + precision + " NULL"
}

func (p mysql) getSchemaBytesType(f *StructField) string {
	size := util.Int64Value(f.Size)

	if size > 0 && size < 16384 {
		return fmt.Sprintf("varbinary(%d)", size)
	}

	f.Size = nil
	if size >= 16384 && size < 65536 {
		return "blob"
	}

	if size >= 65536 && size < int64(math.Pow(2, 24)) {
		return "mediumblob"
	}

	// size >= int64(math.Pow(2, 24)) || size <= 0
	return "longblob"
}

func (p mysql) FullDataTypeOf(field *StructField) string {
	SQL := field.DriverDataType

	if field.NotNull != nil && *field.NotNull {
		SQL += " NOT NULL"
	}

	if field.Unique != nil && *field.Unique {
		SQL += " UNIQUE"
	}

	if field.DefaultValue != "" {
		SQL += " DEFAULT " + field.DefaultValue
	}
	return SQL
}

// AutoMigrate
func (p *mysql) AutoMigrate(ctx context.Context, sample interface{}, opts ...QueryOption) error {
	if len(opts) == 0 {
		opts = DefaultMysqlTableOptions
	}
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return err
	}

	if !p.HasTable(ctx, o.Table()) {
		return p.CreateTable(ctx, o)
	}

	actualFields, _ := p.ColumnTypes(ctx, o)

	expectFields := GetFields(o.Sample(), p)

	for _, expectField := range expectFields.Fields {
		var foundField *StructField

		for _, v := range actualFields {
			if v.Name == expectField.Name {
				foundField = &v
				break
			}
		}

		if foundField == nil {
			// not found, add column
			if err := p.AddColumn(ctx, expectField.Name, o); err != nil {
				return err
			}
		} else if err := p.MigrateColumn(ctx, expectField, foundField, o); err != nil {
			// found, smart migrate
			return err
		}
	}

	// index
	for _, f := range expectFields.Fields {
		if f.IndexKey && !p.HasIndex(ctx, f.Name, o) {
			if err := p.CreateIndex(ctx, f.Name, o); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *mysql) GetTables(ctx context.Context) (tableList []string, err error) {
	err = p.Query(ctx, "SELECT TABLE_NAME FROM information_schema.tables WHERE TABLE_SCHEMA=?", p.CurrentDatabase(ctx)).Rows(&tableList)
	return
}

func (p *mysql) CreateTable(ctx context.Context, o *queryOptions) (err error) {
	var (
		SQL                     = "CREATE TABLE `" + o.Table() + "` ("
		hasPrimaryKeyInDataType bool
	)

	fields := GetFields(o.Sample(), p)
	for _, f := range fields.Fields {
		hasPrimaryKeyInDataType = hasPrimaryKeyInDataType || strings.Contains(strings.ToUpper(f.DriverDataType), "PRIMARY KEY")
		SQL += fmt.Sprintf("`%s` %s,", f.Name, p.FullDataTypeOf(f))
	}

	{
		primaryKeys := []string{}
		for _, f := range fields.Fields {
			if f.PrimaryKey {
				primaryKeys = append(primaryKeys, "`"+f.Name+"`")
			}
		}

		if len(primaryKeys) > 0 {
			SQL += fmt.Sprintf("PRIMARY KEY (%s),", strings.Join(primaryKeys, ","))
		}
	}

	var autoIncrementNum int64
	for _, f := range fields.Fields {
		if f.AutoIncrement && f.AutoIncrementNum > 0 {
			autoIncrementNum = f.AutoIncrementNum
		}

		if !f.IndexKey {
			continue
		}
		if f.IndexClass != "" {
			SQL += f.IndexClass + " "
		}

		SQL += "INDEX "
		if f.IndexName != "" {
			SQL += "`" + f.IndexName + "` "
		}
		SQL += "(`" + f.Name + "`) "

		if f.IndexComment != "" {
			SQL += fmt.Sprintf(" COMMENT '%s'", f.IndexComment)
		}

		if f.IndexOption != "" {
			SQL += " " + f.IndexOption
		}

		SQL += ","
	}

	SQL = strings.TrimSuffix(SQL, ",")

	SQL += ")"

	for _, v := range o.tableOptions {
		SQL += " " + v
	}

	if _, err = p.Exec(ctx, SQL); err != nil {
		return err
	}

	if autoIncrementNum > 0 {
		if _, err = p.Exec(ctx, fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", o.Table(), autoIncrementNum)); err != nil {
			return err
		}
	}

	return nil
}

func (p *mysql) DropTable(ctx context.Context, o *queryOptions) error {
	//p.Exec("SET FOREIGN_KEY_CHECKS = 0;")
	_, err := p.Exec(ctx, "DROP TABLE IF EXISTS `"+o.Table()+"`")
	//p.Exec("SET FOREIGN_KEY_CHECKS = 1;")
	return err
}

func (p *mysql) HasTable(ctx context.Context, tableName string) bool {
	var count int64
	p.Query(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema=? AND table_name=? AND table_type=?", p.CurrentDatabase(ctx), tableName, "BASE TABLE").Row(&count)

	return count > 0
}

func (p *mysql) AddColumn(ctx context.Context, field string, o *queryOptions) error {
	// avoid using the same name field
	f := GetField(o.Sample(), field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec(ctx, "ALTER TABLE `"+o.Table()+"` ADD `"+f.Name+"` "+p.FullDataTypeOf(f))

	return err
}

func (p *mysql) DropColumn(ctx context.Context, field string, o *queryOptions) error {
	_, err := p.Exec(ctx, "ALTER TABLE `"+o.Table()+"` DROP COLUMN `"+field+"`")
	return err
}

func (p *mysql) AlterColumn(ctx context.Context, field string, o *queryOptions) error {
	f := GetField(o.Sample(), field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec(ctx, "ALTER TABLE `"+o.Table()+"` MODIFY COLUMN `"+f.Name+"` "+p.FullDataTypeOf(f))
	return err
}

func (p *mysql) HasColumn(ctx context.Context, field string, o *queryOptions) bool {
	var count int64
	p.Query(ctx, "SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_schema=? AND table_name=? AND column_name=?",
		p.CurrentDatabase(ctx), o.Table(), field,
	).Row(&count)

	return count > 0
}

// field: 1 - expect
// columntype: 2 - actual
func (p *mysql) MigrateColumn(ctx context.Context, expect, actual *StructField, o *queryOptions) error {
	alterColumn := false

	// check size
	if expect.Size != nil && actual.Size != nil && util.Int64Value(expect.Size) != util.Int64Value(actual.Size) {
		klog.V(3).InfoS("migrate", "column", expect.Name, "expect", util.Int64Value(expect.Size), "actiual", util.Int64Value(actual.Size))
		alterColumn = true
	}

	// check nullable
	if expect.NotNull != nil && util.BoolValue(expect.NotNull) != util.BoolValue(actual.NotNull) {
		klog.V(3).InfoS("migrate.nullable", "column", expect.Name,
			"expect", util.BoolValue(expect.NotNull), "actiual", util.BoolValue(actual.NotNull))
		alterColumn = true
	}

	if alterColumn {
		return p.AlterColumn(ctx, expect.Name, o)
	}

	return nil
}

// ColumnTypes return columnTypes []gColumnType and execErr error
func (p *mysql) ColumnTypes(ctx context.Context, o *queryOptions) ([]StructField, error) {
	query := "SELECT column_name, is_nullable, data_type, character_maximum_length, numeric_precision, numeric_scale FROM information_schema.columns WHERE table_schema=? AND table_name=?"

	columns := []mysqlColumn{}
	err := p.Query(ctx, query, p.CurrentDatabase(ctx), o.Table()).Rows(&columns)
	if err != nil {
		return nil, err
	}

	columnTypes := []StructField{}
	for _, c := range columns {
		columnTypes = append(columnTypes, c.FiledOptions())
	}

	return columnTypes, nil

}

func (p *mysql) CreateIndex(ctx context.Context, name string, o *queryOptions) error {
	f := GetField(o.Sample(), name, p)
	if f == nil {
		return fmt.Errorf("failed to create index with name %s", name)
	}

	createIndexSQL := "CREATE "
	if f.Class != "" {
		createIndexSQL += f.Class + " "
	}
	createIndexSQL += fmt.Sprintf("INDEX `%s` ON %s(%s)", f.Name, o.Table(), f.Name)

	_, err := p.Exec(ctx, createIndexSQL)
	return err

}

func (p *mysql) DropIndex(ctx context.Context, name string, o *queryOptions) error {
	_, err := p.Exec(ctx, fmt.Sprintf("DROP INDEX `%s` ON `%s`", name, o.Table()))
	return err
}

func (p *mysql) HasIndex(ctx context.Context, name string, o *queryOptions) bool {
	var count int64
	p.Query(ctx, "SELECT count(*) FROM information_schema.statistics WHERE table_schema=? AND table_name=? AND index_name=?",
		p.CurrentDatabase(ctx), o.Table(), name).Row(&count)

	return count > 0
}

func (p *mysql) CurrentDatabase(ctx context.Context) (name string) {
	p.Query(ctx, "SELECT DATABASE()").Row(&name)
	return
}
