package apifunc

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/torm"
)

type ContextApiFunc struct {
	_Api     Api
	_Torms   torm.Torms
	_Project Project
}

//ConvertContext2ContextApiFunc 从ctx 中获取 ContextApiFunc上下文
func ConvertContext2ContextApiFunc(ctx context.Context) (contextApiFunc *ContextApiFunc, err error) {

	contextApiFunc, ok := ctx.(*ContextApiFunc)
	if ok {
		return contextApiFunc, nil
	}
	err = errors.Errorf("ctx not impliment of ContextApiFunc")
	return nil, err
}

func NewContextApiFunc(api Api, torms torm.Torms, project Project) (contextApiFunc *ContextApiFunc) {
	contextApiFunc = &ContextApiFunc{
		_Api:     api,
		_Torms:   torms,
		_Project: project,
	}
	return contextApiFunc
}

func (*ContextApiFunc) Deadline() (deadline time.Time, ok bool) {
	return
}

func (*ContextApiFunc) Done() <-chan struct{} {
	return nil
}

func (*ContextApiFunc) Err() error {
	return nil
}

func (*ContextApiFunc) Value(key any) any {
	return nil
}

//RunApiFunc 执行ApiFunc 之所有不写成  func  (cApiFunc ContextApiFunc)Run(input []byte) (out []byte, err error) 是因为ContextApiFunc 作为数据传入到脚本中，为脚本提供上下文资源，在脚本中不能调用Run方法
func RunApiFunc(ctxApiFunc *ContextApiFunc, input []byte) (out []byte, err error) {
	out, err = ctxApiFunc._Api.Run(ctxApiFunc, input)
	if err != nil && ctxApiFunc._Api.ErrorHandler != nil {
		out = ctxApiFunc._Api.ErrorHandler(ctxApiFunc, err)
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (ctxApiFunc *ContextApiFunc) RunTransferByFunc(funcname string, input []byte) (out []byte, err error) {
	project := ctxApiFunc._Project
	scriptEngine, err := project._ScriptEngines.GetByLanguage(project.CurrentLanguage)
	if err != nil {
		return nil, err
	}
	out, err = TransferByFunc(project.FuncTransfers, scriptEngine, funcname, input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (ctxApiFunc *ContextApiFunc) RunTorm(tormName string, input []byte) (out []byte, err error) {
	tor, err := ctxApiFunc._Torms.GetByTplName(tormName)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	out, err = tor.Run(ctx, input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (ctxApiFunc *ContextApiFunc) Torms() (torms torm.Torms) {
	return ctxApiFunc._Torms
}

func (ctxApiFunc *ContextApiFunc) Project() (project Project) {
	return ctxApiFunc._Project
}
