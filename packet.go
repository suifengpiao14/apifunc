package apifunc

import (
	"context"
	"reflect"

	"github.com/suifengpiao14/packethandler"
)

type _ApiFlowFuncPacketHandler struct {
	funcName       string
	dynamicLogicFn BusinessFlowFn
}

const (
	PACKETHANDLER_NAME_API_FLOW = "github.com/suifengpiao14/apifunc/_ApiFlowFuncPacketHandler"
)

func NewApiLogicFuncPacketHandler(funcName string, dynamicLogicFn BusinessFlowFn) (packHandler packethandler.PacketHandlerI) {
	return &_ApiFlowFuncPacketHandler{
		funcName:       funcName,
		dynamicLogicFn: dynamicLogicFn,
	}
}

func (packet *_ApiFlowFuncPacketHandler) Name() string {
	return PACKETHANDLER_NAME_API_FLOW
}

func (packet *_ApiFlowFuncPacketHandler) Description() string {
	return `脚本中逻辑处理函数`
}
func (packet *_ApiFlowFuncPacketHandler) Before(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
	contextApiFunc, err := ConvertContext2ContextApiFunc(ctx)
	if err != nil {
		return ctx, nil, err
	}
	if packet.dynamicLogicFn != nil {
		out, err = packet.dynamicLogicFn(contextApiFunc, input)
		if err != nil {
			return ctx, nil, err
		}
		return ctx, out, nil
	}
	language := contextApiFunc._Project.CurrentLanguage
	engine, err := contextApiFunc._Project._ScriptEngines.GetByLanguage(language)
	if err != nil {
		return ctx, out, nil
	}
	if packet.funcName == "" {
		return ctx, input, nil
	}
	dstType := reflect.TypeOf((BusinessFlowFn)(nil))
	dstSymbol, err := engine.GetSymbolFromScript(packet.funcName, dstType) //延迟获取(不在NewFuncPacketHandler 中获取，方便在此之前修改动态脚本内容)
	if err != nil {
		return ctx, nil, err
	}
	logicFn := dstSymbol.Interface().(BusinessFlowFn) // 一定能转换，否则前面就报错了
	out, err = logicFn(contextApiFunc, input)
	if err != nil {
		return ctx, nil, err
	}
	return newCtx, out, nil
}
func (packet *_ApiFlowFuncPacketHandler) After(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
	err = packethandler.ERROR_EMPTY_FUNC
	return newCtx, input, err
}

func (packet *_ApiFlowFuncPacketHandler) String() string {
	return ""
}
