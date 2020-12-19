package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	restful "github.com/emicklei/go-restful"
	"github.com/yubo/golib/configer"
	"github.com/yubo/golib/openapi"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/proc"
	"github.com/yubo/golib/util"
	"k8s.io/klog/v2"
)

const (
	moduleName = "sys.audit"
)

type Module struct {
	name      string
	db        *orm.Db
	http      proc.HttpServer
	getFilter func(acl string) (restful.FilterFunction, string, error)
}

var (
	errOrmDb      = errors.New("unable get orm db")
	errHttpServer = errors.New("unable get http server")
	_module       = &Module{name: moduleName}
	hookOps       = []proc.HookOps{{
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_START,
		Priority: proc.PRI_POST_MODULE,
		// dblooger need get user.filter, so, private -> post_module
	}, {
		// reload.represtart
		Hook:     _module.preStartHook,
		Owner:    moduleName,
		HookNum:  proc.ACTION_RELOAD,
		Priority: proc.PRI_POST_MODULE,
	}}
)

func (p *Module) preStartHook(ops *proc.HookOps, configer *configer.Configer) error {
	popts := ops.Options()

	if p.db = popts.Db(); p.db == nil {
		return fmt.Errorf("unable to get db from options")
	}

	if p.http = popts.Http(); p.http == nil {
		return fmt.Errorf("unable to get httpServer from options")
	}

	if auth := popts.Auth(); auth != nil {
		p.getFilter = auth.GetFilter
	}

	// /api/v1/audit
	if err := p.install(); err != nil {
		return err
	}

	popts = popts.SetAudit(p)

	ops.SetOptions(popts)
	return nil
}

/*
-- -----------------------------------------------------
-- Table `log`
-- -----------------------------------------------------
DROP TABLE IF EXISTS `log`;
CREATE TABLE `log` (
  `id`			bigint unsigned				NOT NULL AUTO_INCREMENT,
  `user_name`		varchar(128)		DEFAULT ''	NOT NULL,
  `target`		varchar(128)		DEFAULT ''	NOT NULL,
  `action`		varchar(64)		DEFAULT ''	NOT NULL,
  `peer_addr`		varchar(64)		DEFAULT ''	NOT NULL,
  `extra`		blob					NULL,
  `err`			blob					NULL,
  `created_at`		integer	unsigned	DEFAULT '0'	NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `index_create_at` (`created_at`)
) ENGINE = InnoDB AUTO_INCREMENT=1000 DEFAULT CHARACTER SET = utf8 COLLATE = utf8_unicode_ci
COMMENT = 'log';
*/

type CreateLogInput struct {
	UserName  *string
	Target    *string
	Action    *string
	PeerAddr  *string
	Extra     *string
	Err       *string
	CreatedAt *int64
}

type Log struct {
	Id        *int64  `json:"id,omitempty"`
	CreatedAt *int64  `json:"createdAt,omitempty" out:",date"`
	UserName  *string `json:"userName,omitempty"`
	Action    *string `json:"action,omitempty"`
	Target    *string `json:"target,omitempty" out:",substr,0,32"`
	Extra     *string `json:"extra,omitempty" out:"-"`
	Err       *string `json:"err,omitempty" out:",substr,0,32"`
	PeerAddr  *string `json:"peerAddr,omitempty"`
}

func (p Log) String() string {
	return util.Prettify(p)
}

type GetLogsCntInput struct {
	UserName *string `param:"query" flags:"user,u," description:"query string for user name field"`
	Action   *string `param:"query" flags:"action,a," description:"query string for action field"`
	Target   *string `param:"query" flags:"target,t," description:"query string for target field"`
	Start    *int64  `param:"query" flags:"-" description:"start timestamp"`
	End      *int64  `param:"query" flags:"-" description:"end timestamp"`
}

type GetLogsInput struct {
	GetLogsCntInput    `param:",inline" flags:",inline"`
	openapi.Pagination `param:",inline" flags:",inline"`
}

type GetLogsOutput struct {
	openapi.RespStatus
	Data struct {
		Total int64 `json: "total"`
		List  []Log `json: "list"`
	} `json:"dat"`
}

type GetLogInput struct {
	Id *int64 `param:"path" flags:"id,," description:"log id, will ignore other param if set this"`
}

type GetLogOutput struct {
	openapi.RespStatus
	Data *Log `json:"dat"`
}

func (p *Module) install() error {
	openapi.SwaggerTagRegister("audit log", "get audit log")

	ws := new(restful.WebService)
	opt := &openapi.WsOption{
		Ws:   ws.Path("/api/v1/audit").Produces(restful.MIME_JSON).Consumes("*/*"),
		Tags: []string{"audit log"},
		Acl:  p.getFilter,
	}

	openapi.WsRouteBuild(opt, []openapi.WsRoute{{
		SubPath: "/", Acl: "read",
		Method: "GET",
		Handle: p.getLogs,
	}, {
		SubPath: "/{id}", Acl: "read",
		Method: "GET",
		Handle: p.getLog,
	}})

	p.http.Add(ws)

	p.http.Filter(p.logFilter)

	return nil
}

type logEntity interface {
	Log() (action, target string, data interface{})
}

func (p *Module) logFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	chain.ProcessFilter(req, resp)

	in, ok := openapi.ReqEntityFrom(req)
	if !ok {
		return
	}

	entity, ok := in.(logEntity)
	if !ok {
		return
	}

	action, target, data := entity.Log()
	if err := p.log(req, &action, &target, data, resp.Error()); err != nil {
		klog.Error(err)
	}
	return
}

// ###########################
func (p *Module) getLogs(req *restful.Request, resp *restful.Response, in *GetLogsInput) (*GetLogsOutput, error) {

	sqlExtra := in.SqlExtra("id desc")

	total, list, err := GetLogs(p.db, in, sqlExtra)

	ret := &GetLogsOutput{RespStatus: openapi.NewRespStatus(err)}
	ret.Data.Total = total
	ret.Data.List = list

	return ret, nil
}

func (p *Module) getLog(req *restful.Request, resp *restful.Response, in *GetLogInput) (*GetLogOutput, error) {
	ret, err := GetLog(p.db, in.Id)

	return &GetLogOutput{
		RespStatus: openapi.NewRespStatus(err),
		Data:       ret,
	}, nil
}

type Extra struct {
	Method    string
	Url       string
	PeerAddr  string
	TokenName string
	Data      interface{}
}

func (p *Module) Log(UserName, Target, Action, PeerAddr, Extra, Err string, CreatedAt int64) error {
	return CreateLog(p.db, &CreateLogInput{
		UserName:  &UserName,
		Target:    &Target,
		Action:    &Action,
		PeerAddr:  &PeerAddr,
		Extra:     &Extra,
		Err:       &Err,
		CreatedAt: &CreatedAt,
	})
}

func (p *Module) log(req *restful.Request, action, target *string, data interface{}, err error) (e error) {
	token, ok := openapi.TokenFrom(req)
	if !ok {
		token = &openapi.AnonymousToken{}
	}

	addr := util.GetIPAddress(req.Request)

	// data maybe Marshal error
	if _, e := json.Marshal(data); e != nil {
		data = e.Error()
	}

	klog.V(6).Infof("%s, %s, err %v, userName %s, tokenName %s, ADDR %s, data %s",
		req.Request.Method,
		req.SelectedRoutePath(),
		err,
		token.GetUserName(),
		token.GetTokenName(),
		addr,
		util.JsonStr(data),
	)

	return CreateLog(p.db, &CreateLogInput{
		UserName: util.String(token.GetUserName()),
		Target:   target,
		Action:   action,
		PeerAddr: &addr,
		Extra: util.String(util.JsonStr(Extra{
			Method:    req.Request.Method,
			Url:       req.SelectedRoutePath(),
			PeerAddr:  addr,
			TokenName: token.GetTokenName(),
			Data:      data,
		})),
		Err:       util.ErrorString(err),
		CreatedAt: util.NowPtr(),
	})
}

func genLogSql(in *GetLogsInput) (where string, args []interface{}) {
	a := []string{}
	b := []interface{}{}
	if userName := util.StringValue(in.UserName); userName != "" {
		a = append(a, "user_name like ?")
		b = append(b, "%"+userName+"%")
	}
	if action := util.StringValue(in.Action); action != "" {
		a = append(a, "action like ?")
		b = append(b, "%"+action+"%")
	}
	if target := util.StringValue(in.Target); target != "" {
		a = append(a, "target like ?")
		b = append(b, "%"+target+"%")
	}
	if start := util.Int64Value(in.Start); start > 0 {
		a = append(a, "created_at > ?")
		b = append(b, start)
	}
	if end := util.Int64Value(in.End); end > 0 {
		a = append(a, "created_at < ?")
		b = append(b, end)
	}
	if len(a) > 0 {
		where = " where " + strings.Join(a, " and ")
		args = b
	}
	return
}

func GetLogsCnt(db *orm.Db, in *GetLogsInput) (ret int64, err error) {
	sql, args := genLogSql(in)
	err = db.Query("select count(*) from log"+sql, args...).Row(&ret)
	return
}

func GetLogs(db *orm.Db, in *GetLogsInput, sqlExtra string) (total int64, ret []Log, err error) {
	sql, args := genLogSql(in)
	if err = db.Query("select count(*) from log"+sql, args...).Row(&total); err != nil {
		return
	}
	err = db.Query("select * from log"+sql+sqlExtra, args...).Rows(&ret)
	return
}

func GetLog(db *orm.Db, id *int64) (ret *Log, err error) {
	err = db.Query("select * from log where id = ?", util.Int64Value(id)).Row(&ret)
	return
}

func CreateLog(db *orm.Db, in *CreateLogInput) error {
	return db.Insert("log", in)
}

func init() {
	proc.RegisterHooks(hookOps)
}
