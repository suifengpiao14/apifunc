package apifunc

import (
	"context"
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

// RegisterAPIHandler 业务逻辑处理函数需要提前注册，方便编译时从这里取
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

// RegisterAPIByModel 通过模型注册路由
func (c *Container) RegisterAPIByModel(transferFuncModels TransferFuncModels, apiModels ApiModels, sourceModels SourceModels, tormModels TormModels) (err error) {

	sources := make(torm.Sources, 0)
	for _, sourceModel := range sourceModels {
		source, err := torm.MakeSource(sourceModel.SourceID, sourceModel.SourceType, sourceModel.Config, sourceModel.SSHConfig, sourceModel.DDL)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}

	for _, apiModel := range apiModels {
		setting, err := makeSetting(apiModel, sources, tormModels)
		if err != nil {
			return err
		}
		logicFn, err := c.GetDynamicLogicFn(apiModel.Route, apiModel.Method)
		if errors.Is(err, ERROR_NOT_FOUND_DYNAMIC_LOGICFN_FUNC) {
			err = nil
			logicFn = ApiHandlerEmptyFn() // 所有的都以empty 做保底逻辑
			tormName := setting.Api.Dependents.FilterByType(Dependent_Type_Torm).First()
			if tormName != "" {
				tor, err := setting.Torms.GetByName(tormName)
				if err != nil {
					return err
				}
				logicFn = ApiHandlerRunTormFn(*tor)
			}

		}
		if err != nil {
			return err
		}
		setting.BusinessLogicFn = logicFn
		capi, err := NewApiCompiled(setting)
		if err != nil {
			return err
		}
		c.RegisterAPI(capi)
	}
	return nil
}

func makeSetting(apiModel ApiModel, sources torm.Sources, tormModels TormModels) (setting *Setting, err error) {
	flows := packethandler.Flow(strings.Split(strings.TrimSpace(apiModel.Flow), ","))
	flows.DropEmpty()
	if len(flows) == 0 {
		flows = DefaultAPIFlows
	}

	transfers := apiModel.PathTransferLine.Transfer()
	inTransfers, outTransfers := transfers.GetByNamespace(apiModel.ApiId).SplitInOut()
	setting = &Setting{
		Api: Api{
			ApiName:             apiModel.ApiId,
			Method:              strings.TrimSpace(apiModel.Method),
			Route:               strings.TrimSpace(apiModel.Route),
			RequestLineschema:   strings.TrimSpace(apiModel.InputSchema),
			ResponseLineschema:  strings.TrimSpace(apiModel.OutputSchema),
			InputPathTransfers:  inTransfers,
			OutputPathTransfers: outTransfers,
			Flow:                flows,
		},

		Torms: make(torm.Torms, 0),
	}

	dependents, err := apiModel.Dependents.Dependents()
	if err != nil {
		return nil, err
	}
	setting.Api.Dependents = dependents
	tormNames := dependents.FilterByType("torm").Fullnames()
	subTormModel := tormModels.GetByName(tormNames...)
	groupdTormModels := subTormModel.GroupBySourceId()
	for sourceId, tormModels := range groupdTormModels {
		source, err := sources.GetByIdentifer(sourceId)
		if err != nil {
			return nil, err
		}
		tplStr := tormModels.GetTpl()
		for _, tormModel := range tormModels {
			flows := packethandler.Flow(strings.Split(strings.TrimSpace(tormModel.Flow), ","))
			flows.DropEmpty()
			if len(flows) == 0 {
				flows = DefaultTormFlows
			}
			torms, err := torm.ParserTpl(source, tplStr, tormModel.TransferLine, flows, nil)
			if err != nil {
				return nil, err
			}
			setting.Torms.Add(torms...)
		}
	}
	return setting, nil
}

// RegisterRouteFn 给router 注册路由
func (c *Container) RegisterRouteFn(routeFn func(method string, path string)) {
	for _, api := range c.apis {
		routeFn(api._api.Method, api._api.Route)
	}
}

// ApiHandlerRunTormFn 内置运行单个Torm业务逻辑函数
func ApiHandlerRunTormFn(tor torm.Torm) (logicHandler DynamicLogicFn) {
	tormName := tor.Name
	logicHandler = func(ctx context.Context, injectObject InjectObject, input []byte) (out []byte, err error) {
		switch strings.ToUpper(tor.Source.Type) {
		case torm.SOURCE_TYPE_SQL:
			out, err = injectObject.ExecSQLTPL(ctx, tormName, input)
			if err != nil {
				return nil, err
			}
			return out, nil
		default:
			err = errors.Errorf("not implement source type:%s", torm.SOURCE_TYPE_SQL)
			return nil, err
		}
	}
	return logicHandler
}

// ApiHandlerEmptyFn 内置空业务逻辑函数，使用mock数据
func ApiHandlerEmptyFn() (logicHandler DynamicLogicFn) {
	logicHandler = func(ctx context.Context, injectObject InjectObject, input []byte) (out []byte, err error) {
		return out, nil
	}
	return logicHandler
}
