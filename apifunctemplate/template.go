package apifunctemplate

import (
	"context"

	"strings"
	"text/template"

	"bytes"

	"github.com/Masterminds/sprig/v3"
	"github.com/d5/tengo/v2"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/tengolib/tengocontext"
	"github.com/suifengpiao14/tengolib/util"
	gormLogger "gorm.io/gorm/logger"
)

type LogName string

func (l LogName) String() string {
	return string(l)
}

const (
	LOG_INFO_SQL_TEMPLATE LogName = "LogInfoTemplateSQL"
)

type LogInfoTemplateSQL struct {
	Context context.Context
	SQL     string      `json:"sql"`
	Named   string      `json:"named"`
	Data    interface{} `json:"data"`
	Result  string      `json:"result"`
	Err     error       `json:"error"`
}

func (l LogInfoTemplateSQL) GetName() logchan.LogName {
	return LOG_INFO_SQL_TEMPLATE
}
func (l LogInfoTemplateSQL) Error() error {
	return l.Err
}

func (l LogInfoTemplateSQL) BeforeSend() {
	return
}

func (l LogInfoTemplateSQL) GetContext() (ctx context.Context) {
	return l.Context
}

func (l LogInfoTemplateSQL) SetContext(ctx context.Context) {
	l.Context = ctx
}

const (
	EOF                  = "\n"
	WINDOW_EOF           = "\r\n"
	HTTP_HEAD_BODY_DELIM = EOF + EOF
)

type TemplateOut struct {
	tengo.ObjectImpl
	Out  string                 `json:"out"`
	Data map[string]interface{} `json:"data"`
}

func (to *TemplateOut) TypeName() string {
	return "template-out"
}
func (to *TemplateOut) String() string {
	return to.Out
}

func (to *TemplateOut) ToSQL(args ...tengo.Object) (sqlObj tengo.Object, err error) {
	sqlLogInfo := LogInfoTemplateSQL{}
	defer func() {
		sqlLogInfo.Err = err
		logchan.SendLogInfo(sqlLogInfo)
	}()
	if len(args) != 0 {
		return nil, tengo.ErrWrongNumArguments
	}
	ctxObj, ok := args[0].(*tengocontext.TengoContext)
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "context",
			Expected: "context.Context",
			Found:    args[0].TypeName(),
		}
	}
	sqlStr, err := ToSQL(to.Out, to.Data)
	if err != nil {
		return nil, err
	}
	sqlLogInfo.Context = ctxObj.Context
	sqlObj = &tengo.String{Value: sqlStr}
	sqlLogInfo.SQL = sqlStr
	return sqlObj, nil
}

//TemplateFuncMap 外部需要增加模板自定义函数时,在初始化模板前,设置该变量即可
var TemplateFuncMap = make([]template.FuncMap, 0)

type ApifuncTemplate struct {
	Template *template.Template
	TPL      string
}

func NewTemplate() (t *template.Template) {
	t = template.New("").Funcs(TemplatefuncMapSQL).Funcs(sprig.TxtFuncMap())
	for _, fnMap := range TemplateFuncMap {
		t = t.Funcs(fnMap)
	}
	return t
}

func (t *ApifuncTemplate) TypeName() string {
	return "template"
}
func (t *ApifuncTemplate) String() string {
	return t.TPL
}

func (t *ApifuncTemplate) Exec(tplName string, volume VolumeInterface) (out string, changedVolume VolumeInterface, err error) {
	var b bytes.Buffer
	err = t.Template.ExecuteTemplate(&b, tplName, volume)
	if err != nil {
		err = errors.WithStack(err)
		return "", nil, err
	}
	out = strings.ReplaceAll(b.String(), WINDOW_EOF, EOF)
	out = util.TrimSpaces(out)
	return out, volume, nil
}

func GetTemplateNames(t *template.Template) []string {
	out := make([]string, 0)
	for _, tpl := range t.Templates() {
		name := tpl.Name()
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

//ToSQL 将字符串、数据整合为sql
func ToSQL(named string, data map[string]interface{}) (sql string, err error) {
	statment, arguments, err := sqlx.Named(named, data)
	if err != nil {
		err = errors.WithStack(err)
		return "", err
	}
	sql = gormLogger.ExplainSQL(statment, nil, `'`, arguments...)
	return sql, nil
}
