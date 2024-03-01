package apifunc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
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
