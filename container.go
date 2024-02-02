package apifunc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/suifengpiao14/logchan/v2"
)

// 容器，包含所有预备的资源、脚本等
type Container struct {
	apis     map[string]*apiCompiled
	lockCApi sync.Mutex
}

func NewContainer(logFn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) (container *Container) {
	container = &Container{
		apis:     map[string]*apiCompiled{},
		lockCApi: sync.Mutex{},
	}
	container.setLogger(logFn) // 外部注入日志处理组件
	return container
}

func (c *Container) RegisterAPI(capi *apiCompiled) {
	c.lockCApi.Lock()
	defer c.lockCApi.Unlock()
	methods := make([]string, 0)
	if capi.Method != "" {
		methods = strings.Split(capi.Method, ",")
	}
	capi._container = c // 关联容器
	for _, method := range methods {
		key := apiMapKey(capi.Route, method)
		c.apis[key] = capi
	}
}

// 计算api map key
func apiMapKey(route, method string) (key string) {
	key = strings.ToLower(fmt.Sprintf("%s_%s", route, method))
	return key
}

func (c *Container) GetCApi(route string, method string) (capi *apiCompiled, ok bool) {
	key := apiMapKey(route, method)
	c.lockCApi.Lock()
	defer c.lockCApi.Unlock()
	capi, ok = c.apis[key]
	return capi, ok
}

// SsetLogger 封装相关性——全局设置 功能
func (c *Container) setLogger(fn func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error)) {
	logchan.SetLoggerWriter(fn)
}
