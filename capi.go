package apifunc

import (
	"context"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/goscript"
	"github.com/suifengpiao14/packethandler"
	"github.com/suifengpiao14/pathtransfer"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
	"github.com/tidwall/gjson"
)

var DefaultAPIFlows = packethandler.Flow{
	lineschemapacket.PACKETHANDLER_NAME_ValidatePacket,
	lineschemapacket.PACKETHANDLER_NAME_MergeDefaultPacketHandler,
	lineschemapacket.PACKETHANDLER_NAME_TransferTypeFormatPacket,
	packet.PACKETHANDLER_NAME_TransferPacketHandler,
	packet.PACKETHANDLER_NAME_JsonMergeInputToOutputPacket,
	PACKETHANDLER_NAME_API_FLOW,
}

var DefaultTormFlows = packethandler.Flow{
	packet.PACKETHANDLER_NAME_TransferPacketHandler,
	packet.PACKETHANDLER_NAME_TormPackHandler,
	packet.PACKETHANDLER_NAME_CUDEvent,
	packet.PACKETHANDLER_NAME_MysqlPacketHandler,
}

// ExecSouceFn 执行资源函数
type ExecSouceFn func(ctx context.Context, identify string, input []byte) (out []byte, err error)

func TransferByFunc(funcTransfers pathtransfer.Transfers, scriptEngine goscript.ScriptI, funcname string, input []byte) (out []byte, err error) {
	funcTransfers = funcTransfers.GetByNamespace(pathtransfer.JoinPath(pathtransfer.Transfer_Top_Namespace_Func, funcname).String())
	funcInTransfers, funcOutTransfers := funcTransfers.SplitInOut()
	funcInput := gjson.GetBytes(input, funcInTransfers.Reverse().GjsonPath()).String()
	callScript := scriptEngine.CallFuncScript(funcname, funcInput)
	funcReturn, err := scriptEngine.Run(callScript)
	if err != nil {
		return nil, err
	}
	funcOut := gjson.Get(funcReturn, funcOutTransfers.GjsonPath()).String()
	out, err = jsonpatch.MergePatch(input, []byte(funcOut))
	if err != nil {
		return nil, err
	}
	return out, nil
}

type BusinessFlowFn func(ctxApiFunc *ContextApiFunc, input []byte) (out []byte, err error)

type Project struct {
	CurrentLanguage string
	FuncTransfers   pathtransfer.Transfers
	Scripts         goscript.Scripts
	_ScriptEngines  goscript.ScriptIs
}

func (pro *Project) AddScriptEngine(scriptEngines ...goscript.ScriptI) {
	pro._ScriptEngines.AddReplace(scriptEngines...)
}
func (pro *Project) ScriptEngines() (scriptEngines goscript.ScriptIs) {
	return pro._ScriptEngines
}

func (pro *Project) Init() (err error) {
	for language, scripts := range pro.Scripts.GroupByLanguage() {
		engine, err := pro._ScriptEngines.GetByLanguage(language) // 优先使用项目配置
		if errors.Is(err, goscript.ERROR_NOT_FOUND_SCRIPTI_BY_LANGUAGE) {
			engine, err = goscript.NewScriptEngine(language)
			if err != nil {
				return err
			}
		}
		for _, script := range scripts {
			engine.WriteCode(script.Code)
		}
		callScript, err := pro.FuncTransfers.GetCallFnScript(language)
		if err != nil {
			return err
		}
		engine.WriteCode(callScript)
		pro._ScriptEngines.AddReplace(engine)
	}
	return nil
}

type Api struct {
	ApiName             string                 `json:"apiName"`
	Route               string                 `json:"route"`
	Method              string                 `json:"method"`
	Flow                packethandler.Flow     `json:"flow"` // 按顺序组装执行流程
	businessFlowFn      BusinessFlowFn         // 业务处理流程函数
	RequestLineschema   string                 `json:"requestLineschema"`
	ResponseLineschema  string                 `json:"responseLineschema"`
	ResponseDefaultJson string                 `json:"responseDefaultJson"` // 返回数据默认值,一般填充协议字段如: code,message
	PathTransfers       pathtransfer.Transfers `json:"pathTransfers"`
	ErrorHandler        stream.ErrorHandler
	PacketHandlers      packethandler.PacketHandlers
}

func (api Api) Key() string {
	return apiKey(api.Route, api.Method)
}

// EqualFold 判断2个api是否相同（名称或者路由和方法一致，则判断为相同）
func (api Api) EqualFold(api1 Api) (ok bool) {
	return strings.EqualFold(api.ApiName, api1.ApiName) || strings.EqualFold(api.Key(), api1.Key())
}

func apiKey(route string, method string) string {
	return fmt.Sprintf("%s_%s", strings.ToLower(route), strings.ToLower(method))
}

// Merge 和并2个api属性（有时基于代码本身会先注册logic 函数，再加载其它配置，此时需要合并属性，和并的2个api要么名称相同，要么 route和method相同）
func (api *Api) Merge(mergedApi Api) (err error) {
	if !api.EqualFold(mergedApi) {
		err = errors.Errorf("the two APIs used for merging must have the same name or key,want:%s|%s,got:%s|%s",
			api.ApiName,
			mergedApi.ApiName,
			api.Key(),
			mergedApi.Key(),
		)
		return err
	}
	if api.ApiName == "" {
		api.ApiName = mergedApi.ApiName
	}
	if api.Route == "" {
		api.Route = mergedApi.Route
	}
	if api.Method == "" {
		api.Method = mergedApi.Method
	}
	if len(api.Flow) == 0 {
		api.Flow = mergedApi.Flow
	}
	if api.RequestLineschema == "" {
		api.RequestLineschema = mergedApi.RequestLineschema
	}
	if api.ResponseLineschema == "" {
		api.ResponseLineschema = mergedApi.ResponseLineschema
	}
	if len(api.PathTransfers) == 0 {
		api.PathTransfers = mergedApi.PathTransfers
	}

	if api.ErrorHandler == nil {
		api.ErrorHandler = mergedApi.ErrorHandler
	}
	if api.businessFlowFn == nil {
		api.businessFlowFn = mergedApi.businessFlowFn
	}
	//处理器池，采用合并方式
	packethandlers := make(packethandler.PacketHandlers, 0)
	packethandlers.AddReplace(mergedApi.PacketHandlers...)
	packethandlers.AddReplace(api.PacketHandlers...)
	api.PacketHandlers = packethandlers

	return nil
}

type Apis []Api

func (aps *Apis) AddMerge(apis ...Api) {
	for _, ap := range apis {
		exists := false
		for i, ap0 := range *aps {
			if ap0.EqualFold(ap) {
				exists = true
				(*aps)[i].Merge(ap) //忽略错误，应为已经判定相同
				break

			}
		}
		if !exists {
			(*aps) = append(*aps, ap)
		}
	}
}

func (aps Apis) GetByName(name string) (api *Api, err error) {
	for _, api := range aps {
		if strings.EqualFold(api.ApiName, name) {
			return &api, nil
		}
	}
	return nil, ERROR_NOT_FOUND_API
}

func (aps Apis) GetByRoute(route string, method string) (api *Api, err error) {
	for _, api := range aps {
		if strings.EqualFold(api.Key(), apiKey(route, method)) {
			return &api, nil
		}
	}
	return nil, ERROR_NOT_FOUND_API
}

func (api Api) GetRoute() (mehtod string, path string) {
	return api.Method, api.Route
}
func (api Api) UnpackSchema() (lineschema string) {
	return api.RequestLineschema
}
func (api Api) PackSchema() (lineschema string) {
	return api.ResponseLineschema
}

func (api *Api) Init() (err error) {
	if api.ApiName == "" {
		err = errors.Errorf("api name required,api key:%s", api.Key())
		return err
	}
	if strings.EqualFold(api.Key(), apiKey("", "")) {
		err = errors.Errorf("api method,path required,api name:%s", api.Key())
		return err
	}
	err = api.InitPacketHandler()
	if err != nil {
		return err
	}
	api.Flow.DropEmpty()
	return
}

func (api *Api) InitPacketHandler() (err error) {
	packetHandlers := packethandler.NewPacketHandlers()
	//验证、格式化 入参
	unpackClineschema, packClineschema, err := lineschemapacket.ParserLineschemaPacket2Clineschema(api)
	if err != nil {
		err = errors.WithMessage(err, "lineschemapacket.ParserLineschemaPacket2Clineschema")
		return err
	}
	packClineschema.DefaultJson = []byte(api.ResponseDefaultJson) //只设置协议字段默认值
	lineschemaPacketHandlers := lineschemapacket.ServerpacketHandlers(*unpackClineschema, *packClineschema)
	packetHandlers.Append(lineschemaPacketHandlers...)
	packetHandlers.Append(packet.NewJsonMergeInputPacket())                                                                                //增加合并输入数据
	namespaceInput := fmt.Sprintf("%s%s%s", pathtransfer.Transfer_Top_Namespace_API, api.ApiName, pathtransfer.Transfer_Direction_input)   // 去除命名空间
	namespaceOutput := fmt.Sprintf("%s%s%s", pathtransfer.Transfer_Top_Namespace_API, api.ApiName, pathtransfer.Transfer_Direction_output) // 去除命名空间
	inputPathTransfers, outputPathTransfers := api.PathTransfers.SplitInOut()
	inputTransfer := inputPathTransfers.ModifySrcPath(func(path pathtransfer.Path) (newPath pathtransfer.Path) {
		return path.TrimNamespace(namespaceInput)
	}).GjsonPath()
	outputTransfer := outputPathTransfers.ModifySrcPath(func(path pathtransfer.Path) (newPath pathtransfer.Path) {
		return path.TrimNamespace(namespaceOutput)
	}).Reverse().GjsonPath()

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(inputTransfer, outputTransfer)
	packetHandlers.Append(transferHandler)
	//注入逻辑处理函数
	logicFuncName := fmt.Sprintf("%s%s", ApiLogicFuncNamePrefix, api.ApiName)
	packetHandlers.Append(NewApiLogicFuncPacketHandler(logicFuncName, api.businessFlowFn))
	api.PacketHandlers = packetHandlers

	return
}

func (api Api) Run(ctx context.Context, input []byte) (out []byte, err error) {
	packetHandlers, err := api.PacketHandlers.GetByName(api.Flow...)
	if err != nil {
		return nil, err
	}
	s := stream.NewStream(api.ApiName, nil, packetHandlers...)
	out, err = s.Run(ctx, input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// 对提取变量有意义，注释暂时保留，另外可以通过注释关联资源，避免关联信息分散，增强关联性，方便维护
// func TemplateVariables(tmpl *template.Template) []string {
// 	var variables []string
// 	// 访问模板树的根节点
// 	rootNodes := tmpl.Templates()
// 	// 遍历模板树的根节点
// 	for _, t := range rootNodes {
// 		rootNode := t.Tree.Root
// 		for _, node := range rootNode.Nodes {
// 			fmt.Println(node.String())
// 			// 检查节点类型是否为变量
// 			if node.Type() == parse.NodeAction && len(node.String()) > 2 && node.String()[0] == '.' {
// 				// 提取变量名称
// 				variableName := node.String()[2:]
// 				variables = append(variables, variableName)
// 			}
// 		}
// 	}

// 	return variables
// }

/*
	 type apiCompiled struct {
		_api       Api
		_container *Container
		_Torms     torm.Torms
	}
*/
var ERROR_COMPILED_API = errors.New("compiled api error")

var ApiLogicFuncNamePrefix = "script.ApiLogic" //api 逻辑函数名前缀

/* func NewApiCompiled(setting *Setting) (capi *apiCompiled, err error) {
	defer func() {
		if err != nil {
			err = errors.WithMessage(err, ERROR_COMPILED_API.Error())
		}
	}()
	capi = &apiCompiled{
		_api:   setting.Api,
		_Torms: setting.Torms,
	}

	project := setting.Project
	capi._api._Project = project

	return capi, nil
}
*/
// Run 执行API
/* func (capi *apiCompiled) Run(ctx context.Context, inputJson string) (out string, err error) {
	handlers, err := capi._api.PacketHandlers.GetByName(capi._api.Flow...)
	if err != nil {
		return "", err
	}
	outB, err := handlers.Run(ctx, []byte(inputJson))
	if err != nil {
		if capi._api.ErrorHandler == nil {
			return "", err
		}
		outB = capi._api.ErrorHandler(ctx, err) // error 处理
		err = nil
	}
	out = string(outB)
	return out, nil
}
*/
