package orm

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/yubo/golib/util"
)

type mysqlColumn struct {
	ColumnName             string
	IsNullable             sql.NullString
	Datatype               string
	CharacterMaximumLength sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
}

func (p *mysqlColumn) FiledOptions() FieldOptions {
	ret := FieldOptions{
		name:           p.ColumnName,
		driverDataType: p.Datatype,
	}

	if p.CharacterMaximumLength.Valid {
		ret.size = util.Int64(p.CharacterMaximumLength.Int64)
	}

	if p.IsNullable.Valid {
		ret.notNull = util.Bool(p.IsNullable.String != "YES")
	}

	return ret
}

var _ Driver = &Mysql{}

// Mysql m struct
type Mysql struct {
	DB
}

func (p *Mysql) ParseField(f *field) {
	f.driverDataType = p.driverDataTypeOf(f)
}

func (p *Mysql) driverDataTypeOf(f *field) string {
	switch f.dataType {
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

	return string(f.dataType)
}

func (p *Mysql) getSchemaIntAndUnitType(f *field) string {
	sqlType := "bigint"

	switch size := util.Int64Value(f.size); {
	case size <= 8:
		sqlType = "tinyint"
	case size <= 16:
		sqlType = "smallint"
	case size <= 24:
		sqlType = "mediumint"
	case size <= 32:
		sqlType = "int"
	}

	if f.dataType == Uint {
		sqlType += " unsigned"
	}

	if f.autoIncrement {
		sqlType += " AUTO_INCREMENT"
	}

	return sqlType
}
func (p *Mysql) getSchemaFloatType(f *field) string {
	size := util.Int64Value(f.size)

	if size <= 32 {
		return "float"
	}

	return "double"
}

func (p *Mysql) getSchemaStringType(f *field) string {
	size := util.Int64Value(f.size)

	if size == 0 {
		if DefaultStringSize > 0 {
			size = int64(DefaultStringSize)
		} else {
			hasIndex := f.Has("index") || f.Has("unique")
			// TEXT, GEOMETRY or JSON column can't have a default value
			if f.primaryKey || f.hasDefaultValue || hasIndex {
				size = 191 // utf8mb4
			}
		}
	}

	if size >= 65536 && size <= int64(math.Pow(2, 24)) {
		return "mediumtext"
	}

	if size > int64(math.Pow(2, 24)) || size <= 0 {
		return "longtext"
	}

	f.size = util.Int64(size)

	return fmt.Sprintf("varchar(%d)", size)
}

func (p Mysql) getSchemaTimeType(field *field) string {
	precision := ""

	if field.precision != nil && *field.precision > 0 {
		precision = fmt.Sprintf("(%d)", *field.precision)
	}

	if util.BoolValue(field.notNull) || field.primaryKey {
		return "datetime" + precision
	}
	return "datetime" + precision + " NULL"
}

func (p Mysql) getSchemaBytesType(f *field) string {
	size := util.Int64Value(f.size)

	if size > 0 && size < 65536 {
		return fmt.Sprintf("varbinary(%d)", size)
	}

	if size >= 65536 && size <= int64(math.Pow(2, 24)) {
		return "mediumblob"
	}

	return "longblob"
}

func (p Mysql) FullDataTypeOf(field *FieldOptions) string {
	SQL := field.driverDataType

	if field.notNull != nil && *field.notNull {
		SQL += " NOT NULL"
	}

	if field.unique != nil && *field.unique {
		SQL += " UNIQUE"
	}

	if field.defaultValue != "" {
		SQL += " DEFAULT " + field.defaultValue
	}
	return SQL
}

// AutoMigrate
func (p *Mysql) AutoMigrate(sample interface{}, opts ...SqlOption) error {
	o, err := sqlOptions(sample, opts)
	if err != nil {
		return err
	}

	if !p.HasTable(o.Table()) {
		return p.CreateTable(o)
	}

	actualFields, _ := p.ColumnTypes(o)

	expectFields := tableFields(o.sample, p)

	for _, expectField := range expectFields.list {
		var foundField *FieldOptions

		for _, v := range actualFields {
			if v.name == expectField.name {
				foundField = &v
				break
			}
		}

		if foundField == nil {
			// not found, add column
			if err := p.AddColumn(expectField.name, o); err != nil {
				return err
			}
		} else if err := p.MigrateColumn(expectField.FieldOptions, foundField, o); err != nil {
			// found, smart migrate
			return err
		}
	}

	// index
	for _, f := range expectFields.list {
		if f.indexKey && !p.HasIndex(f.name, o) {
			if err := p.CreateIndex(f.name, o); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Mysql) GetTables() (tableList []string, err error) {
	err = p.DB.Query("SELECT TABLE_NAME FROM information_schema.tables where TABLE_SCHEMA=?", p.CurrentDatabase()).Rows(&tableList)
	return
}

func (p *Mysql) CreateTable(o *SqlOptions) (err error) {
	var (
		SQL                     = "CREATE TABLE `" + o.Table() + "` ("
		hasPrimaryKeyInDataType bool
	)

	fields := tableFields(o.sample, p)
	for _, f := range fields.list {
		hasPrimaryKeyInDataType = hasPrimaryKeyInDataType || strings.Contains(strings.ToUpper(f.driverDataType), "PRIMARY KEY")
		SQL += fmt.Sprintf("%s %s,", f.name, p.FullDataTypeOf(f.FieldOptions))
	}

	{
		primaryKeys := []string{}
		for _, f := range fields.list {
			if f.primaryKey {
				primaryKeys = append(primaryKeys, "`"+f.name+"`")
			}
		}

		if len(primaryKeys) > 0 {
			SQL += fmt.Sprintf("PRIMARY KEY (%s),", strings.Join(primaryKeys, ","))
		}
	}

	for _, f := range fields.list {
		if !f.indexKey {
			continue
		}
		if f.idxClass != "" {
			SQL += f.idxClass + " "
		}
		SQL += "INDEX `" + f.name + "`,"

		if f.idxComment != "" {
			SQL += fmt.Sprintf(" COMMENT '%s'", f.idxComment)
		}

		if f.idxOption != "" {
			SQL += " " + f.idxOption
		}

		SQL += ","
	}

	SQL = strings.TrimSuffix(SQL, ",")

	SQL += ")"

	_, err = p.Exec(SQL)

	return err
}

func (p *Mysql) DropTable(o *SqlOptions) error {
	p.Exec("SET FOREIGN_KEY_CHECKS = 0;")
	_, err := p.Exec("DROP TABLE IF EXISTS `" + o.Table() + "`")
	p.Exec("SET FOREIGN_KEY_CHECKS = 1;")
	return err
}

func (p *Mysql) HasTable(tableName string) bool {
	var count int64
	p.Query("SELECT count(*) FROM information_schema.tables WHERE table_schema=? AND table_name=? AND table_type=?", p.CurrentDatabase(), tableName, "BASE TABLE").Row(&count)

	return count > 0
}

func (p *Mysql) AddColumn(field string, o *SqlOptions) error {
	// avoid using the same name field
	f := tableFieldLookup(o.sample, field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` ADD `" + f.name + "` " + p.FullDataTypeOf(f))

	return err
}

func (p *Mysql) DropColumn(field string, o *SqlOptions) error {
	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` DROP COLUMN `" + field + "`")
	return err
}

func (p *Mysql) AlterColumn(field string, o *SqlOptions) error {
	f := tableFieldLookup(o.sample, field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` MODIFY COLUMN `" + f.name + "` " + p.FullDataTypeOf(f))
	return err
}

func (p *Mysql) HasColumn(field string, o *SqlOptions) bool {
	var count int64
	p.Query("SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_schema=? AND table_name=? AND column_name=?",
		p.CurrentDatabase(), o.Table(), field,
	).Row(&count)

	return count > 0
}

// field: 1 - expect
// columntype: 2 - actual
func (p *Mysql) MigrateColumn(expect, actual *FieldOptions, o *SqlOptions) error {
	alterColumn := false

	// check size
	if actual.size != nil && util.Int64Value(expect.size) != util.Int64Value(actual.size) {
		fmt.Printf("%s.size %v != %v\n",
			expect.name,
			util.Int64Value(expect.size),
			util.Int64Value(actual.size),
		)
		alterColumn = true
	}

	// check nullable
	if expect.notNull != nil && util.BoolValue(expect.notNull) != util.BoolValue(actual.notNull) {
		fmt.Printf("%s.notnull %v != %v\n", expect.name, expect.notNull, actual.notNull)
		alterColumn = true
	}

	if alterColumn {
		return p.AlterColumn(expect.name, o)
	}

	return nil
}

// ColumnTypes return columnTypes []gorm.ColumnType and execErr error
func (p *Mysql) ColumnTypes(o *SqlOptions) ([]FieldOptions, error) {
	query := "SELECT column_name, is_nullable, data_type, character_maximum_length, numeric_precision, numeric_scale FROM information_schema.columns WHERE table_schema=? AND table_name=?"

	columns := []mysqlColumn{}
	err := p.Query(query, p.CurrentDatabase(), o.Table()).Rows(&columns)
	if err != nil {
		return nil, err
	}

	columnTypes := []FieldOptions{}
	for _, c := range columns {
		columnTypes = append(columnTypes, c.FiledOptions())
	}

	return columnTypes, nil

}

func (p *Mysql) CreateIndex(name string, o *SqlOptions) error {
	f := tableFieldLookup(o.sample, name, p)
	if f == nil {
		return fmt.Errorf("failed to create index with name %s", name)
	}

	createIndexSQL := "CREATE "
	if f.class != "" {
		createIndexSQL += f.class + " "
	}
	createIndexSQL += fmt.Sprintf("INDEX `%s` ON %s(%s)", f.name, o.Table(), f.name)

	_, err := p.Exec(createIndexSQL)
	return err

}

func (p *Mysql) DropIndex(name string, o *SqlOptions) error {
	_, err := p.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", name, o.Table()))
	return err
}

func (p *Mysql) HasIndex(name string, o *SqlOptions) bool {
	var count int64
	p.Query("SELECT count(*) FROM information_schema.statistics WHERE table_schema=? AND table_name=? AND index_name=?",
		p.CurrentDatabase(), o.Table(), name).Row(&count)

	return count > 0
}

func (p *Mysql) CurrentDatabase() (name string) {
	p.Query("SELECT DATABASE()").Row(&name)
	return
}

func init() {
	Register("mysql", func(db DB) Driver {
		return &Mysql{DB: db}
	})
}
