package apifunc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/packethandler"
	"github.com/suifengpiao14/torm"
)

// 容器，包含所有预备的资源、脚本等
type Container struct {
	apis              map[string]*apiCompiled
	dynamicLogicFnMap map[string]DynamicLogicFn
	lockCApi          sync.Mutex
}

func NewContainer(logFn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) (container *Container) {
	container = &Container{
		apis:              map[string]*apiCompiled{},
		lockCApi:          sync.Mutex{},
		dynamicLogicFnMap: map[string]DynamicLogicFn{},
	}
	container.setLogger(logFn) // 外部注入日志处理组件
	return container
}

func (c *Container) RegisterAPI(capi *apiCompiled) {
	c.lockCApi.Lock()
	defer c.lockCApi.Unlock()
	methods := make([]string, 0)
	if capi._api.Method != "" {
		methods = strings.Split(capi._api.Method, ",")
	}
	capi._container = c // 关联容器
	for _, method := range methods {
		key := apiMapKey(capi._api.Route, method)
		c.apis[key] = capi
	}
}

//RegisterAPIHandler 业务逻辑处理函数需要提前注册，方便编译时从这里取
func (c *Container) RegisterAPIHandler(method string, route string, logicHandler DynamicLogicFn) {
	c.lockCApi.Lock()
	defer c.lockCApi.Unlock()
	key := apiMapKey(route, method)
	c.dynamicLogicFnMap[key] = logicHandler
}

var ERROR_NOT_FOUND_DYNAMIC_LOGICFN_FUNC = errors.New("not found dynamic logic func")

func (c *Container) GetDynamicLogicFn(route string, method string) (logicHandler DynamicLogicFn, err error) {
	key := apiMapKey(route, method)
	logicHandler, ok := c.dynamicLogicFnMap[key]
	if !ok {
		err = errors.WithMessagef(ERROR_NOT_FOUND_DYNAMIC_LOGICFN_FUNC, "route:%s,method:%s", route, method)
		return nil, err
	}
	return logicHandler, nil
}

// 计算api map key
func apiMapKey(route, method string) (key string) {
	key = strings.ToLower(fmt.Sprintf("%s_%s", route, method))
	return key
}

var ERROR_NOT_FOUND_API = errors.New("not found api")

func (c *Container) GetCApi(route string, method string) (capi *apiCompiled, err error) {
	key := apiMapKey(route, method)
	capi, ok := c.apis[key]
	if !ok {
		err = errors.WithMessagef(ERROR_NOT_FOUND_API, "route:%s,method:%s", route, method)
		if err != nil {
			return nil, err
		}
	}
	return capi, nil
}

// SsetLogger 封装相关性——全局设置 功能
func (c *Container) setLogger(fn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) {
	logchan.SetLoggerWriter(fn)
}

//RegisterAPIByModel 通过模型注册路由
func (c *Container) RegisterAPIByModel(apiModels ApiModels, sourceModels SourceModels, tormModels TormModels) (err error) {

	sources := make(torm.Sources, 0)
	for _, sourceModel := range sourceModels {
		source, err := torm.MakeSource(sourceModel.SourceID, sourceModel.SourceType, sourceModel.Config, sourceModel.DDL)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}

	for _, apiModel := range apiModels {
		logicFn, err := c.GetDynamicLogicFn(apiModel.Route, apiModel.Method)
		if err != nil {
			return err
		}
		setting, err := makeSetting(apiModel, sources, tormModels, logicFn)
		if err != nil {
			return err
		}
		capi, err := NewApiCompiled(setting)
		if err != nil {
			return err
		}
		c.RegisterAPI(capi)
	}
	return nil
}

func makeSetting(apiModel ApiModel, sources torm.Sources, tormModels TormModels, logicFn DynamicLogicFn) (setting *Setting, err error) {
	flows := packethandler.ToFlows(strings.Split(strings.TrimSpace(apiModel.Flows), ",")...)
	if len(flows) == 0 {
		flows = DefaultAPIFlows
	}
	transfers := apiModel.PathTransferLine.Transfer()
	inTransfers, outTransfers := transfers.SplitInOut(apiModel.ApiId)
	setting = &Setting{
		Api: Api{
			ApiName:             apiModel.ApiId,
			Method:              strings.TrimSpace(apiModel.Method),
			Route:               strings.TrimSpace(apiModel.Route),
			RequestLineschema:   strings.TrimSpace(apiModel.InputSchema),
			ResponseLineschema:  strings.TrimSpace(apiModel.OutputSchema),
			InputPathTransfers:  inTransfers,
			OutputPathTransfers: outTransfers,
			Flows:               flows,
		},

		Torms:           make(torm.Torms, 0),
		BusinessLogicFn: logicFn,
	}

	dependents, err := apiModel.Dependents.Dependents()
	if err != nil {
		return nil, err
	}
	tormNames := dependents.FilterByType("torm").Fullnames()
	subTormModel := tormModels.GetByName(tormNames...)
	groupdTormModels := subTormModel.GroupBySourceId()
	for sourceId, tormModels := range groupdTormModels {
		source, err := sources.GetByIdentifer(sourceId)
		if err != nil {
			return nil, err
		}
		for _, tormModel := range tormModels {
			flows := packethandler.ToFlows(strings.Split(strings.TrimSpace(tormModel.Flows), ",")...)
			if len(flows) == 0 {
				flows = DefaultTormFlows
			}
			torms, err := torm.ParserTpl(source, tormModel.Tpl, tormModel.PathTransferLine, flows, nil)
			if err != nil {
				return nil, err
			}
			setting.Torms.Add(torms...)
		}
	}
	return setting, nil
}
