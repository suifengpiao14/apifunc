package apifunc

import (
	"context"
	"reflect"

	"github.com/suifengpiao14/packethandler"
)

type _ApiLogicFuncPacketHandler struct {
	funcName       string
	contextApiFunc ContextApiFunc
}

func NewApiLogicFuncPacketHandler(funcName string, contextApiFunc ContextApiFunc) (packHandler packethandler.PacketHandlerI) {
	return &_ApiLogicFuncPacketHandler{
		funcName:       funcName,
		contextApiFunc: contextApiFunc,
	}
}

func (packet *_ApiLogicFuncPacketHandler) Name() string {
	return packet.funcName
}

func (packet *_ApiLogicFuncPacketHandler) Description() string {
	return `脚本中逻辑处理函数`
}
func (packet *_ApiLogicFuncPacketHandler) Before(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
	language := packet.contextApiFunc.Project.CurrentLanguage
	engine, err := packet.contextApiFunc.Api._ScriptEngines.GetByLanguage(language)
	if err != nil {
		return ctx, out, nil
	}
	if packet.funcName == "" {
		return ctx, input, nil
	}
	dstType := reflect.TypeOf((DynamicLogicFn)(nil))
	dstSymbol, err := engine.GetSymbolFromScript(packet.funcName, dstType) //延迟获取(不在NewFuncPacketHandler 中获取，方便在此之前修改动态脚本内容)
	if err != nil {
		return ctx, nil, err
	}
	logicFn := dstSymbol.Interface().(DynamicLogicFn) // 一定能转换，否则前面就报错了
	out, err = logicFn(packet.contextApiFunc, input)
	if err != nil {
		return ctx, nil, err
	}
	return newCtx, out, nil
}
func (packet *_ApiLogicFuncPacketHandler) After(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
	err = packethandler.ERROR_EMPTY_FUNC
	return newCtx, out, err
}

func (packet *_ApiLogicFuncPacketHandler) String() string {
	return ""
}
