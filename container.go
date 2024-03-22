package apifunc

import (
	"context"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/goscript"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/pathtransfer"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/torm"
)

// 容器，包含所有预备的资源、脚本等
type Container struct {
	apis     Apis
	project  Project
	torms    torm.Torms
	compiled sync.Once
}

func NewContainer(logFn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) (container *Container) {
	container = &Container{
		apis:  make(Apis, 0),
		torms: make(torm.Torms, 0),
	}
	container.setLogger(logFn) // 外部注入日志处理组件
	return container
}

func (c *Container) RegisterAPI(api Api) {
	c.apis.AddMerge(api)
}

//Compile 只会执行一次
func (c *Container) Compile() (err error) {
	c.compiled.Do(func() {
		//初始化api
		for i := range c.apis {
			err = c.apis[i].Init()
			if err != nil {
				return
			}
		}
		//初始化torm
		for i, tor := range c.torms {
			switch strings.ToUpper(tor.Source.Type) {
			case torm.SOURCE_TYPE_SQL:
				c.torms[i].PacketHandlers, err = packet.TormSQLPacketHandler(tor)
				if err != nil {
					return
				}
			}
		}
		//初始化project
		err = c.project.Init()
		if err != nil {
			return
		}
	})
	if err != nil {
		return err
	}
	return nil
}

// RegisterAPIBusinessFlow 业务流程处理函数需要提前注册，方便编译时从这里取
func (c *Container) RegisterAPIBusinessFlow(method string, route string, logicHandler BusinessFlowFn) {
	api := Api{
		Method:         method,
		Route:          route,
		BusinessFlowFn: logicHandler,
	}
	c.apis.AddMerge(api)
}

var ERROR_NOT_FOUND_API = errors.New("not found api")

//GetContextApiFunc  根据route，method获取特定api执行上下文
func (c *Container) GetContextApiFunc(route string, method string) (contextApiFunc *ContextApiFunc, err error) {
	api, err := c.apis.GetByRoute(route, method)
	if err != nil {
		return nil, err
	}
	contextApiFunc = &ContextApiFunc{
		_Api:     *api,
		_Torms:   c.torms,
		_Project: c.project,
	}
	return contextApiFunc, nil
}

// SsetLogger 封装相关性——全局设置 功能
func (c *Container) setLogger(fn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) {
	logchan.SetLoggerWriter(fn)
}

//RegisterProject 注册项目
func (c *Container) RegisterProject(scriptLanguage string, scriptEngines goscript.ScriptIs, transferFuncModels TransferFuncModels) {
	project := Project{
		CurrentLanguage: scriptLanguage,
		FuncTransfers:   make(pathtransfer.Transfers, 0),
		Scripts:         make(goscript.Scripts, 0),
	}
	for _, transferFuncModel := range transferFuncModels {
		if transferFuncModel.Script != "" {
			script := goscript.Script{
				Language: transferFuncModel.Language,
				Code:     transferFuncModel.Script,
			}
			project.Scripts = append(project.Scripts, script)
		}
		project.FuncTransfers.AddReplace(transferFuncModel.TransferLine.Transfer()...)
	}
	c.project = project
}

//RegisterTorms 注册torm
func (c *Container) RegisterTormByModels(tormModels TormModels, sourceModels SourceModels) (err error) {
	if c.torms == nil {
		c.torms = make(torm.Torms, 0)
	}
	sources := make(torm.Sources, 0)
	for _, sourceModel := range sourceModels {
		source, err := torm.MakeSource(sourceModel.SourceID, sourceModel.SourceType, sourceModel.Config, sourceModel.SSHConfig, sourceModel.DDL)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}
	torms, err := tormModels.Torms(sources)
	if err != nil {
		return err
	}
	c.torms.AddReplace(torms...)
	return nil
}

// RegisterAPIByModel 通过模型注册路由
func (c *Container) RegisterAPIByModel(apiModels ...ApiModel) {
	if c.apis == nil {
		c.apis = make(Apis, 0)
	}
	for _, apiModel := range apiModels {
		api := apiModel.Api()
		c.apis.AddMerge(api)
	}
}

// RegisterRouteFn 给router 注册路由
func (c *Container) RegisterRouteFn(routeFn func(method string, path string)) {
	for _, api := range c.apis {
		routeFn(api.Method, api.Route)
	}
}

// ApiHandlerRunTormFn 内置运行单个Torm业务逻辑函数
func ApiHandlerRunTormFn(tors ...torm.Torm) (logicHandler BusinessFlowFn) {
	logicHandler = func(ctx *ContextApiFunc, input []byte) (out []byte, err error) {
		outputArr := make([][]byte, len(tors))
		for i, tor := range tors {
			switch strings.ToUpper(tor.Source.Type) {
			case torm.SOURCE_TYPE_SQL:
				ctx := context.Background()

				outputArr[i], err = tor.Run(ctx, input)
				if err != nil {
					return nil, err
				}
			default:
				err = errors.Errorf("not implement source type:%s", torm.SOURCE_TYPE_SQL)
				return nil, err
			}
		}
		for _, subOut := range outputArr {
			out, err = jsonpatch.MergePatch(out, subOut)
			if err != nil {
				return nil, err
			}
		}
		return out, nil

	}
	return logicHandler
}

// ApiHandlerEmptyFn 内置空业务逻辑函数，使用mock数据
func ApiHandlerEmptyFn() (logicHandler BusinessFlowFn) {
	logicHandler = func(ctx *ContextApiFunc, input []byte) (out []byte, err error) {
		return out, nil
	}
	return logicHandler
}
