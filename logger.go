package apifunc

import (
	"context"

	"github.com/suifengpiao14/logchan/v2"
)

// 收集依赖包中的日志名称，实现封装细节(减少使用方明确引入本包依赖的包，隐藏细节)，同时方便统日志命名规则、方便集中查找管理
/*****************************************统一管理日志start**********************************************/

type LogInforInterface logchan.LogInforInterface

/*****************************************统一管理日志end**********************************************/
const (
	LOG_INFO_RUN      = "apiCompiled.Run"
	LOG_INFO_RUN_POST = "apiCompiled.Run.post"
)

type LogName string

func (l LogName) String() string {
	return string(l)
}

//RunLogInfo 运行是日志信息
type RunLogInfo struct {
	Context       context.Context `json:"context"`
	Name          string          `json:"name"`
	OriginalInput string          `json:"originalInput"`
	DefaultJson   string          `json:"defaultJson"`
	PreInput      string          `json:"preInput"`
	PreOutput     string          `json:"preOutInput"`
	Out           string          `json:"out"`
	PostOut       interface{}     `json:"postOut"`
	Err           error
}

func (l RunLogInfo) GetName() logchan.LogName {

	return LogName(l.Name)
}

func (l RunLogInfo) Error() error {
	return l.Err
}

func (l RunLogInfo) BeforeSend() {
}

func (l RunLogInfo) GetContext() (ctx context.Context) {
	return l.Context
}

func (l *RunLogInfo) SetContext(ctx context.Context) {
	l.Context = ctx
}
