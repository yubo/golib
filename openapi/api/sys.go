package api

import (
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"github.com/yubo/golib/mail"
	"github.com/yubo/golib/orm"
	"github.com/yubo/golib/util"
)

type MetricsInterface interface {
	GetKeys() []string
	GetValues() []uint64
	GetKvs() map[string]uint64
}

type SysModule interface {
	// core
	GetDb() (*orm.Db, error)
	GetMailConfig() (*mail.Config, error)

	// info
	StatsRegister(name string, s MetricsInterface) error

	// restful
	RestAdd(service *restful.WebService)
	RestFilter(filter restful.FilterFunction)
	HttpHandle(pattern string, handler http.Handler)
	HttpHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))

	// apidocs
	InfoPropsSet(props *spec.InfoProps)
	SwaggerTagsRegister(tags ...spec.Tag)
	SwaggerTagRegister(name, desc string)
	SecuritySchemeRegister(name string, s *spec.SecurityScheme) error
	EnrichSwaggeerObjectSecurity(swo *spec.Swagger)

	// log
	InstallLogWs(aclHandle func(aclName string) (restful.FilterFunction, string, error))
	Log(in *CreateLogInput) error
	Log5(req *restful.Request, action, target *string, data interface{}, err error) error
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
	//Extra     *json.RawMessage `json:"extra,omitempty" out:"-"`
	Extra    *string `json:"extra,omitempty" out:"-"`
	Err      *string `json:"err,omitempty" out:",substr,0,32"`
	PeerAddr *string `json:"peerAddr,omitempty"`
}

func (p Log) String() string {
	return util.Prettify(p)
}

type LogDetail struct {
	Id        *int64  `json:"id,omitempty"`
	CreatedAt *int64  `json:"createdAt,omitempty" out:",date"`
	UserName  *string `json:"userName,omitempty"`
	Action    *string `json:"action,omitempty"`
	Target    *string `json:"target,omitempty"`
	Extra     *string `json:"extra,omitempty"`
	Err       *string `json:"err,omitempty"`
	PeerAddr  *string `json:"peerAddr,omitempty"`
}

type GetLogsCntInput struct {
	UserName *string `param:"query" flags:"user,u," description:"query string for user name field"`
	Action   *string `param:"query" flags:"action,a," description:"query string for action field"`
	Target   *string `param:"query" flags:"target,t," description:"query string for target field"`
	Start    *int64  `param:"query" flags:"-" description:"start timestamp"`
	End      *int64  `param:"query" flags:"-" description:"end timestamp"`
}

type GetLogsInput struct {
	GetLogsCntInput `param:",inline" flags:",inline"`
	Pagination      `param:",inline" flags:",inline"`
}

type GetLogInput struct {
	Id *int64 `param:"path" flags:"id,," description:"log id, will ignore other param if set this"`
}
