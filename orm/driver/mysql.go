package driver

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
)

var _ orm.Driver = &Mysql{}

func RegisterMysql() {
	orm.Register("mysql", func(db orm.Execer) orm.Driver {
		return &Mysql{db}
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

func (p *mysqlColumn) FiledOptions() orm.StructField {
	ret := orm.StructField{
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

// Mysql m struct
type Mysql struct {
	orm.Execer
}

func (p *Mysql) ParseField(f *orm.StructField) {
	f.DriverDataType = p.driverDataTypeOf(f)
}

func (p *Mysql) driverDataTypeOf(f *orm.StructField) string {
	switch f.DataType {
	case orm.Bool:
		return "boolean"
	case orm.Int, orm.Uint:
		return p.getSchemaIntAndUnitType(f)
	case orm.Float:
		return p.getSchemaFloatType(f)
	case orm.String:
		return p.getSchemaStringType(f)
	case orm.Time:
		return p.getSchemaTimeType(f)
	case orm.Bytes:
		return p.getSchemaBytesType(f)
	}

	return string(f.DataType)
}

func (p *Mysql) getSchemaIntAndUnitType(f *orm.StructField) string {
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

	if f.DataType == orm.Uint {
		sqlType += " unsigned"
	}

	if f.AutoIncrement {
		sqlType += " AUTO_INCREMENT"
	}

	return sqlType
}

func (p *Mysql) getSchemaFloatType(f *orm.StructField) string {
	size := util.Int64Value(f.Size)

	if size <= 32 {
		return "float"
	}

	return "double"
}

func (p *Mysql) getSchemaStringType(f *orm.StructField) string {
	size := util.Int64Value(f.Size)

	if size == 0 {
		if orm.DefaultStringSize > 0 {
			size = int64(orm.DefaultStringSize)
		} else {
			hasIndex := f.Has("index") || f.Has("unique")
			// TEXT, GEOMETRY or JSON column can't have a default value
			if f.PrimaryKey || f.HasDefaultValue || hasIndex {
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

	f.Size = util.Int64(size)

	return fmt.Sprintf("varchar(%d)", size)
}

func (p Mysql) getSchemaTimeType(field *orm.StructField) string {
	precision := ""

	if field.Precision != nil && *field.Precision > 0 {
		precision = fmt.Sprintf("(%d)", *field.Precision)
	}

	if util.BoolValue(field.NotNull) || field.PrimaryKey {
		return "datetime" + precision
	}
	return "datetime" + precision + " NULL"
}

func (p Mysql) getSchemaBytesType(f *orm.StructField) string {
	size := util.Int64Value(f.Size)

	if size > 0 && size < 65536 {
		return fmt.Sprintf("varbinary(%d)", size)
	}

	if size >= 65536 && size <= int64(math.Pow(2, 24)) {
		return "mediumblob"
	}

	return "longblob"
}

func (p Mysql) FullDataTypeOf(field *orm.StructField) string {
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
func (p *Mysql) AutoMigrate(sample interface{}, opts ...orm.SqlOption) error {
	o, err := orm.NewSqlOptions(sample, opts)
	if err != nil {
		return err
	}

	if !p.HasTable(o.Table()) {
		return p.CreateTable(o)
	}

	actualFields, _ := p.ColumnTypes(o)

	expectFields := orm.GetFields(o.Sample(), p)

	for _, expectField := range expectFields.Fields {
		var foundField *orm.StructField

		for _, v := range actualFields {
			if v.Name == expectField.Name {
				foundField = &v
				break
			}
		}

		if foundField == nil {
			// not found, add column
			if err := p.AddColumn(expectField.Name, o); err != nil {
				return err
			}
		} else if err := p.MigrateColumn(expectField, foundField, o); err != nil {
			// found, smart migrate
			return err
		}
	}

	// index
	for _, f := range expectFields.Fields {
		if f.IndexKey && !p.HasIndex(f.Name, o) {
			if err := p.CreateIndex(f.Name, o); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Mysql) GetTables() (tableList []string, err error) {
	err = p.Query("SELECT TABLE_NAME FROM information_schema.tables where TABLE_SCHEMA=?", p.CurrentDatabase()).Rows(&tableList)
	return
}

func (p *Mysql) CreateTable(o *orm.SqlOptions) (err error) {
	var (
		SQL                     = "CREATE TABLE `" + o.Table() + "` ("
		hasPrimaryKeyInDataType bool
	)

	fields := orm.GetFields(o.Sample(), p)
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

	for _, f := range fields.Fields {
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

	_, err = p.Exec(SQL)

	return err
}

func (p *Mysql) DropTable(o *orm.SqlOptions) error {
	//p.Exec("SET FOREIGN_KEY_CHECKS = 0;")
	_, err := p.Exec("DROP TABLE IF EXISTS `" + o.Table() + "`")
	//p.Exec("SET FOREIGN_KEY_CHECKS = 1;")
	return err
}

func (p *Mysql) HasTable(tableName string) bool {
	var count int64
	p.Query("SELECT count(*) FROM information_schema.tables WHERE table_schema=? AND table_name=? AND table_type=?", p.CurrentDatabase(), tableName, "BASE TABLE").Row(&count)

	return count > 0
}

func (p *Mysql) AddColumn(field string, o *orm.SqlOptions) error {
	// avoid using the same name field
	f := orm.GetField(o.Sample(), field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` ADD `" + f.Name + "` " + p.FullDataTypeOf(f))

	return err
}

func (p *Mysql) DropColumn(field string, o *orm.SqlOptions) error {
	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` DROP COLUMN `" + field + "`")
	return err
}

func (p *Mysql) AlterColumn(field string, o *orm.SqlOptions) error {
	f := orm.GetField(o.Sample(), field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` MODIFY COLUMN `" + f.Name + "` " + p.FullDataTypeOf(f))
	return err
}

func (p *Mysql) HasColumn(field string, o *orm.SqlOptions) bool {
	var count int64
	p.Query("SELECT count(*) FROM INFORMATION_SCHEMA.columns WHERE table_schema=? AND table_name=? AND column_name=?",
		p.CurrentDatabase(), o.Table(), field,
	).Row(&count)

	return count > 0
}

// field: 1 - expect
// columntype: 2 - actual
func (p *Mysql) MigrateColumn(expect, actual *orm.StructField, o *orm.SqlOptions) error {
	alterColumn := false

	// check size
	if actual.Size != nil && util.Int64Value(expect.Size) != util.Int64Value(actual.Size) {
		fmt.Printf("%s.size %v != %v\n",
			expect.Name,
			util.Int64Value(expect.Size),
			util.Int64Value(actual.Size),
		)
		alterColumn = true
	}

	// check nullable
	if expect.NotNull != nil && util.BoolValue(expect.NotNull) != util.BoolValue(actual.NotNull) {
		fmt.Printf("%s.notnull %v != %v\n", expect.Name, expect.NotNull, actual.NotNull)
		alterColumn = true
	}

	if alterColumn {
		return p.AlterColumn(expect.Name, o)
	}

	return nil
}

// ColumnTypes return columnTypes []gorm.ColumnType and execErr error
func (p *Mysql) ColumnTypes(o *orm.SqlOptions) ([]orm.StructField, error) {
	query := "SELECT column_name, is_nullable, data_type, character_maximum_length, numeric_precision, numeric_scale FROM information_schema.columns WHERE table_schema=? AND table_name=?"

	columns := []mysqlColumn{}
	err := p.Query(query, p.CurrentDatabase(), o.Table()).Rows(&columns)
	if err != nil {
		return nil, err
	}

	columnTypes := []orm.StructField{}
	for _, c := range columns {
		columnTypes = append(columnTypes, c.FiledOptions())
	}

	return columnTypes, nil

}

func (p *Mysql) CreateIndex(name string, o *orm.SqlOptions) error {
	f := orm.GetField(o.Sample(), name, p)
	if f == nil {
		return fmt.Errorf("failed to create index with name %s", name)
	}

	createIndexSQL := "CREATE "
	if f.Class != "" {
		createIndexSQL += f.Class + " "
	}
	createIndexSQL += fmt.Sprintf("INDEX `%s` ON %s(%s)", f.Name, o.Table(), f.Name)

	_, err := p.Exec(createIndexSQL)
	return err

}

func (p *Mysql) DropIndex(name string, o *orm.SqlOptions) error {
	_, err := p.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", name, o.Table()))
	return err
}

func (p *Mysql) HasIndex(name string, o *orm.SqlOptions) bool {
	var count int64
	p.Query("SELECT count(*) FROM information_schema.statistics WHERE table_schema=? AND table_name=? AND index_name=?",
		p.CurrentDatabase(), o.Table(), name).Row(&count)

	return count > 0
}

func (p *Mysql) CurrentDatabase() (name string) {
	p.Query("SELECT DATABASE()").Row(&name)
	return
}
