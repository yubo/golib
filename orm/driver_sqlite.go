package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/yubo/golib/util"
)

type sqliteColumn struct {
	ColumnName             string
	IsNullable             sql.NullString
	Datatype               string
	CharacterMaximumLength sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
}

func (p *sqliteColumn) FiledOptions() StructField {
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

var _ Driver = &sqlite{}

func RegisterSqlite() {
	Register("sqlite3", func(db Execer, opts *DBOptions) Driver {
		return &sqlite{db, opts}
	})
}

// sqlite m struct
type sqlite struct {
	Execer
	*DBOptions
}

// TODO
func (p *sqlite) ParseField(f *StructField) {
	f.DriverDataType = p.driverDataTypeOf(f)
}

func (p *sqlite) driverDataTypeOf(f *StructField) string {
	switch f.DataType {
	case Bool:
		return "numeric"
	case Int, Uint:
		if f.AutoIncrement && !f.PrimaryKey {
			// https://www.sqlite.org/autoinc.html
			return "integer PRIMARY KEY AUTOINCREMENT"
		} else {
			return "integer"
		}
	case Float:
		return "real"
	case String:
		return "text"
	case Time:
		return "datetime"
	case Bytes:
		return "blob"
	}

	return string(f.DataType)
}

func (p *sqlite) FullDataTypeOf(field *StructField) string {
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
func (p *sqlite) AutoMigrate(sample interface{}, opts ...Option) error {
	o, err := NewOptions(append(opts, WithSample(sample))...)
	if err != nil {
		return err
	}

	if !p.HasTable(o.Table()) {
		return p.CreateTable(o)
	}

	actualFields, _ := p.ColumnTypes(o)
	expectFields := GetFields(o.Sample(), p)

	for _, expectField := range expectFields.Fields {
		var foundField *StructField

		for _, acturalField := range actualFields {
			if acturalField.Name == expectField.Name {
				foundField = &acturalField
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

func (p *sqlite) GetTables() (tableList []string, err error) {
	err = p.Query("SELECT name FROM sqlite_master WHERE type=?", "table").Rows(&tableList)
	return
}

func (p *sqlite) CreateTable(o *Options) (err error) {
	var (
		SQL                     = "CREATE TABLE `" + o.Table() + "` ("
		hasPrimaryKeyInDataType bool
		autoIncrementNum        int64
		autoIncrementField      string
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

	for _, f := range fields.Fields {
		if f.AutoIncrement && f.AutoIncrementNum > 0 {
			autoIncrementNum = f.AutoIncrementNum
			autoIncrementField = f.Name
		}

		if !f.IndexKey {
			continue
		}

		defer func(f *StructField) {
			if err == nil {
				err = p.CreateIndex(f.Name, o)
			}
		}(f)
	}

	SQL = strings.TrimSuffix(SQL, ",")

	SQL += ")"

	_, err = p.Exec(SQL)

	if err == nil && autoIncrementNum > 1 {
		id := autoIncrementNum - 1

		if _, err = p.Exec("insert into `"+o.Table()+"` (`"+autoIncrementField+"`) VALUES (?)", id); err != nil {
			return err
		}

		if _, err = p.Exec("delete from `"+o.Table()+"` where `"+autoIncrementField+"`=?", id); err != nil {
			return err
		}
	}

	return err

}

func (p *sqlite) DropTable(o *Options) error {
	_, err := p.Exec("DROP TABLE IF EXISTS `" + o.Table() + "`")
	return err
}

func (p *sqlite) HasTable(tableName string) bool {
	var count int64
	p.Query("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Row(&count)
	//dlog(1, "count %d err %v", count, err)
	return count > 0
}

func (p *sqlite) AddColumn(field string, o *Options) error {
	// avoid using the same name field
	f := GetField(o.Sample(), field, p)
	if f == nil {
		return fmt.Errorf("failed to look up field with name: %s", field)
	}

	_, err := p.Exec("ALTER TABLE `" + o.Table() + "` ADD `" + f.Name + "` " + p.FullDataTypeOf(f))

	return err
}

func (p *sqlite) DropColumn(field string, o *Options) error {
	return p.recreateTable(o, func(rawDDL string) (sql string, sqlArgs []interface{}, err error) {
		name := field

		reg, err := regexp.Compile("(`|'|\"| |\\[)" + name + "(`|'|\"| |\\]) .*?,")
		if err != nil {
			return "", nil, err
		}

		createSQL := reg.ReplaceAllString(rawDDL, "")

		return createSQL, nil, nil
	})
}

func (p *sqlite) AlterColumn(field string, o *Options) error {
	return p.recreateTable(o, func(rawDDL string) (sql string, sqlArgs []interface{}, err error) {
		f := GetField(o.Sample(), field, p)
		if f == nil {
			err = fmt.Errorf("failed to look up field with name: %s", field)
			return
		}

		var reg *regexp.Regexp
		reg, err = regexp.Compile("(`|'|\"| )" + f.Name + "(`|'|\"| ) .*?,")
		if err != nil {
			return
		}

		sql = reg.ReplaceAllString(
			rawDDL,
			fmt.Sprintf("`%v` %s,", f.Name, p.FullDataTypeOf(f)),
		)
		sqlArgs = []interface{}{}

		return
	})
}

func (p *sqlite) HasColumn(name string, o *Options) bool {
	var count int64
	p.Query("SELECT count(*) FROM sqlite_master WHERE type = ? AND tbl_name = ? AND (sql LIKE ? OR sql LIKE ? OR sql LIKE ? OR sql LIKE ? OR sql LIKE ?)",
		"table", o.Table(), `%"`+name+`" %`, `%`+name+` %`, "%`"+name+"`%", "%["+name+"]%", "%\t"+name+"\t%",
	).Row(&count)

	return count > 0
}

// field: 1 - expect
// columntype: 2 - actual
func (p *sqlite) MigrateColumn(expect, actual *StructField, o *Options) error {
	alterColumn := false

	// check size
	if actual.Size != nil && util.Int64Value(expect.Size) != util.Int64Value(actual.Size) {
		alterColumn = true
	}

	// check nullable
	if expect.NotNull != nil && util.BoolValue(expect.NotNull) != util.BoolValue(actual.NotNull) {
		alterColumn = true
	}

	if alterColumn {
		return p.AlterColumn(expect.Name, o)
	}

	return nil
}

// ColumnTypes return columnTypes []gColumnType and execErr error
func (p *sqlite) ColumnTypes(o *Options) ([]StructField, error) {

	rows, err := p.RawDB().Query("SELECT * FROM `" + o.Table() + "` LIMIT 1")
	if err != nil {
		return nil, err
	}

	defer func() {
		rows.Close()
	}()

	var rawColumnTypes []*sql.ColumnType
	rawColumnTypes, err = rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columnTypes := []StructField{}
	for _, c := range rawColumnTypes {
		columnTypes = append(columnTypes, p.convertFieldOptions(c))
	}

	return columnTypes, nil
}

func (p *sqlite) convertFieldOptions(c *sql.ColumnType) StructField {
	ret := StructField{
		Name:           c.Name(),
		DriverDataType: c.DatabaseTypeName(),
	}

	if size, ok := c.Length(); ok {
		ret.Size = util.Int64(size)
	}

	if nullable, ok := c.Nullable(); ok {
		ret.NotNull = util.Bool(!nullable)
	}

	return ret
}

func (p *sqlite) CreateIndex(name string, o *Options) error {
	f := GetField(o.Sample(), name, p)
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

func (p *sqlite) DropIndex(name string, o *Options) error {
	_, err := p.Exec("DROP INDEX `" + name + "`")
	return err
}

func (p *sqlite) HasIndex(name string, o *Options) bool {
	var count int64
	p.Query("SELECT count(*) FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?",
		"index", o.Table(), name).Row(&count)

	return count > 0
}

func (p *sqlite) CurrentDatabase() (name string) {
	var null interface{}
	p.Query("PRAGMA database_list").Row(&null, &name, &null)
	return
}

func (p *sqlite) getRawDDL(table string) (createSQL string, err error) {
	err = p.Query("SELECT sql FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?", "table", table, table).Row(&createSQL)
	return
}

func (p *sqlite) recreateTable(o *Options,
	getCreateSQL func(rawDDL string) (sql string, sqlArgs []interface{}, err error),
) error {
	table := o.Table()

	rawDDL, err := p.getRawDDL(table)
	if err != nil {
		return err
	}

	newTableName := table + "__temp"

	createSQL, sqlArgs, err := getCreateSQL(rawDDL)
	if err != nil {
		return err
	}
	if createSQL == "" {
		return nil
	}

	tableReg, err := regexp.Compile(" ('|`|\"| )" + table + "('|`|\"| ) ")
	if err != nil {
		return err
	}
	createSQL = tableReg.ReplaceAllString(createSQL, fmt.Sprintf(" `%v` ", newTableName))

	createDDL, err := sqliteParseDDL(createSQL)
	if err != nil {
		return err
	}
	columns := createDDL.getColumns()

	if _, err := p.Exec(createSQL, sqlArgs...); err != nil {
		return err
	}

	queries := []string{
		fmt.Sprintf("INSERT INTO `%v`(%v) SELECT %v FROM `%v`", newTableName, strings.Join(columns, ","), strings.Join(columns, ","), table),
		fmt.Sprintf("DROP TABLE `%v`", table),
		fmt.Sprintf("ALTER TABLE `%v` RENAME TO `%v`", newTableName, table),
	}
	for _, query := range queries {
		if _, err := p.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

type sqliteDDL struct {
	head   string
	fields []string
}

func sqliteParseDDL(sql string) (*sqliteDDL, error) {
	reg := regexp.MustCompile("(?i)(CREATE TABLE [\"`]?[\\w\\d]+[\"`]?)(?: \\((.*)\\))?")
	sections := reg.FindStringSubmatch(sql)

	if sections == nil {
		return nil, errors.New("invalid DDL")
	}

	ddlHead := sections[1]
	ddlBody := sections[2]
	ddlBodyRunes := []rune(ddlBody)
	fields := []string{}

	bracketLevel := 0
	var quote rune = 0
	buf := ""

	for i := 0; i < len(ddlBodyRunes); i++ {
		c := ddlBodyRunes[i]
		var next rune = 0
		if i+1 < len(ddlBodyRunes) {
			next = ddlBodyRunes[i+1]
		}

		if c == '\'' || c == '"' || c == '`' {
			if c == next {
				// Skip escaped quote
				buf += string(c)
				i++
			} else if quote > 0 {
				quote = 0
			} else {
				quote = c
			}
		} else if quote == 0 {
			if c == '(' {
				bracketLevel++
			} else if c == ')' {
				bracketLevel--
			} else if bracketLevel == 0 {
				if c == ',' {
					fields = append(fields, strings.TrimSpace(buf))
					buf = ""
					continue
				}
			}
		}

		if bracketLevel < 0 {
			return nil, errors.New("invalid DDL, unbalanced brackets")
		}

		buf += string(c)
	}

	if bracketLevel != 0 {
		return nil, errors.New("invalid DDL, unbalanced brackets")
	}

	if buf != "" {
		fields = append(fields, strings.TrimSpace(buf))
	}

	return &sqliteDDL{head: ddlHead, fields: fields}, nil
}

func (p *sqliteDDL) compile() string {
	if len(p.fields) == 0 {
		return p.head
	}

	return fmt.Sprintf("%s (%s)", p.head, strings.Join(p.fields, ","))
}

func (p *sqliteDDL) addConstraint(name string, sql string) {
	reg := regexp.MustCompile("^CONSTRAINT [\"`]?" + regexp.QuoteMeta(name) + "[\"` ]")

	for i := 0; i < len(p.fields); i++ {
		if reg.MatchString(p.fields[i]) {
			p.fields[i] = sql
			return
		}
	}

	p.fields = append(p.fields, sql)
}

func (p *sqliteDDL) removeConstraint(name string) bool {
	reg := regexp.MustCompile("^CONSTRAINT [\"`]?" + regexp.QuoteMeta(name) + "[\"` ]")

	for i := 0; i < len(p.fields); i++ {
		if reg.MatchString(p.fields[i]) {
			p.fields = append(p.fields[:i], p.fields[i+1:]...)
			return true
		}
	}
	return false
}

func (p *sqliteDDL) hasConstraint(name string) bool {
	reg := regexp.MustCompile("^CONSTRAINT [\"`]?" + regexp.QuoteMeta(name) + "[\"` ]")

	for _, f := range p.fields {
		if reg.MatchString(f) {
			return true
		}
	}
	return false
}

func (p *sqliteDDL) getColumns() []string {
	res := []string{}

	for _, f := range p.fields {
		fUpper := strings.ToUpper(f)
		if strings.HasPrefix(fUpper, "PRIMARY KEY") ||
			strings.HasPrefix(fUpper, "CHECK") ||
			strings.HasPrefix(fUpper, "CONSTRAINT") {
			continue
		}

		reg := regexp.MustCompile("^[\"`]?([\\w\\d]+)[\"`]?")
		match := reg.FindStringSubmatch(f)

		if match != nil {
			res = append(res, "`"+match[1]+"`")
		}
	}
	return res
}
