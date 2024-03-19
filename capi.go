package apifunc

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/goscript"
	"github.com/suifengpiao14/packethandler"
	"github.com/suifengpiao14/pathtransfer"
	"github.com/suifengpiao14/sqlexec"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
	"github.com/suifengpiao14/torm"
	"github.com/tidwall/gjson"
)

var DefaultAPIFlows = packethandler.Flow{
	lineschemapacket.PACKETHANDLER_NAME_ValidatePacket,
	lineschemapacket.PACKETHANDLER_NAME_MergeDefaultPacketHandler,
	lineschemapacket.PACKETHANDLER_NAME_TransferTypeFormatPacket,
	packet.PACKETHANDLER_NAME_TransferPacketHandler,
	packet.PACKETHANDLER_NAME_JsonMergeInputPacket,
	PACKETHANDLER_NAME_API_LOGIC,
}

var DefaultTormFlows = packethandler.Flow{
	packet.PACKETHANDLER_NAME_TransferPacketHandler,
	packet.PACKETHANDLER_NAME_TormPackHandler,
	packet.PACKETHANDLER_NAME_CUDEvent,
	packet.PACKETHANDLER_NAME_MysqlPacketHandler,
}

const (
	VARIABLE_STORAGE = "storage"
)

type ContextKeyType string

const (
	CONTEXT_KEY_STORAGE = ContextKeyType(VARIABLE_STORAGE)
)

// ExecSouceFn 执行资源函数
type ExecSouceFn func(ctx context.Context, identify string, input []byte) (out []byte, err error)

type InjectObject struct {
	ExecSQLTPL    ExecSouceFn
	PathTransfers pathtransfer.Transfers
	Api           Api
}

func (injectObject InjectObject) ConvertInput(namespace string, input []byte) (out []byte) {
	pathTransfers := injectObject.PathTransfers.GetByNamespace(namespace).Reverse()
	gjsonpath := pathTransfers.ModifyDstPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(strings.TrimPrefix(path, namespace), ".")
		return newPath
	}).GjsonPath()
	outStr := gjson.GetBytes(input, gjsonpath).String()
	out = []byte(outStr)
	return out
}

func (injectObject InjectObject) ConvertOutput(namespace string, data []byte) (out []byte) {
	pathTransfers := injectObject.PathTransfers.GetByNamespace(namespace)
	gjsonpath := pathTransfers.ModifySrcPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(strings.TrimPrefix(path, namespace), ".")
		return newPath
	}).GjsonPath()
	outStr := gjson.GetBytes(data, gjsonpath).String()
	out = []byte(outStr)
	return out
}

func (injectObject InjectObject) TransferByFunc(funcname string, input []byte) (out []byte, err error) {
	scriptEngine, err := injectObject.Api._ScriptEngines.GetByLanguage(injectObject.Api._Project.CurrentLanguage)
	if err != nil {
		return nil, err
	}
	out, err = TransferByFunc(injectObject.Api._Project.FuncTransfers, scriptEngine, funcname, input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func TransferByFunc(funcTransfers pathtransfer.Transfers, scriptEngine goscript.ScriptI, funcname string, input []byte) (out []byte, err error) {
	funcTransfers = funcTransfers.GetByNamespace(pathtransfer.JoinPath(pathtransfer.Transfer_Top_Namespace_Func, funcname))
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

type DynamicLogicFn func(contextApiFunc ContextApiFunc, input []byte) (out []byte, err error)

// Setting 配置
type Setting struct {
	Api     Api        `json:"api"`
	Torms   torm.Torms `json:"torms"` // 模板、资源配置（执行模板后调用什么资源）这里存在模板名称和资源绑定关系，需要在配置中提现这种关系
	Project Project    `json:"project"`
}

type Project struct {
	CurrentLanguage string
	FuncTransfers   pathtransfer.Transfers
	Scripts         goscript.Scripts
	ScriptEngines   goscript.ScriptIs
}

type Api struct {
	ApiName             string                 `json:"apiName"`
	Route               string                 `json:"route"`
	Method              string                 `json:"method"`
	Flow                packethandler.Flow     `json:"flow"` // 按顺序组装执行流程
	BusinessLogicFn     DynamicLogicFn         // 业务处理函数
	RequestLineschema   string                 `json:"requestLineschema"`
	ResponseLineschema  string                 `json:"responseLineschema"`
	InputPathTransfers  pathtransfer.Transfers `json:"inputPathTransfers"`
	OutputPathTransfers pathtransfer.Transfers `json:"outPathTransfers"`
	ErrorHandler        stream.ErrorHandler
	Dependents          Dependents `json:"dependents"`
	_ScriptEngines      goscript.ScriptIs
	_Project            Project
	ContextApiFunc      *ContextApiFunc
	PacketHandlers      packethandler.PacketHandlers
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
	err = api.InitPacketHandler()
	if err != nil {
		return err
	}
	api.Flow.DropEmpty()
	return
}

func (api *Api) InitPacketHandler() (err error) {
	packetHandlers := packethandler.NewPacketHandlers()
	err = lineschemapacket.RegisterLineschemaPacket(api)
	if err != nil {
		return err
	}
	//验证、格式化 入参
	lineschemaPacketHandlers, err := lineschemapacket.ServerpacketHandlers(api)
	if err != nil {
		err = errors.WithMessage(err, "lineschemapacket.ServerpacketHandlers")
		return err
	}

	packetHandlers.Append(lineschemaPacketHandlers...)
	packetHandlers.Append(packet.NewJsonMergeInputPacket())                                     //增加合并输入数据
	namespaceInput := fmt.Sprintf("%s%s", api.ApiName, pathtransfer.Transfer_Direction_input)   // 去除命名空间
	namespaceOutput := fmt.Sprintf("%s%s", api.ApiName, pathtransfer.Transfer_Direction_output) // 去除命名空间
	inputTransfer := api.InputPathTransfers.ModifySrcPath(func(path string) (newPath string) {
		return pathtransfer.TrimNamespace(path, namespaceInput)
	}).GjsonPath()
	outputTransfer := api.OutputPathTransfers.ModifySrcPath(func(path string) (newPath string) {
		return pathtransfer.TrimNamespace(path, namespaceOutput)
	}).Reverse().GjsonPath()

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(inputTransfer, outputTransfer)
	packetHandlers.Append(transferHandler)
	//注入逻辑处理函数
	if api.BusinessLogicFn == nil {
		logicFuncName := fmt.Sprint("%s%s", ApiLogicFuncNamePrefix, api.ApiName)
		if api.ContextApiFunc == nil {
			err = errors.Errorf("api.ContextApiFunc required")
			return err
		}
		packetHandlers.Append(NewApiLogicFuncPacketHandler(logicFuncName, *api.ContextApiFunc))
	}

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

type apiCompiled struct {
	template   *template.Template
	_api       Api
	_container *Container
	_Torms     torm.Torms
}

var ERROR_COMPILED_API = errors.New("compiled api error")

const (
	PACKETHANDLER_NAME_API_LOGIC = "api_logic"
)

var ApiLogicFuncNamePrefix = "script.ApiLogic" //api 逻辑函数名前缀

func NewApiCompiled(setting *Setting) (capi *apiCompiled, err error) {
	defer func() {
		if err != nil {
			err = errors.WithMessage(err, ERROR_COMPILED_API.Error())
		}
	}()
	capi = &apiCompiled{
		_api:     setting.Api,
		_Torms:   setting.Torms,
		template: torm.NewTemplate(),
	}
	//注册模板
	capi.template, err = setting.Torms.Template()
	if err != nil {
		return nil, err
	}

	err = lineschemapacket.RegisterLineschemaPacket(setting.Api)
	if err != nil {
		return nil, err
	}

	packetHandler := packethandler.NewPacketHandlers()
	//验证、格式化 入参
	lineschemaPacketHandlers, err := lineschemapacket.ServerpacketHandlers(setting.Api)
	if err != nil {
		err = errors.WithMessage(err, "lineschemapacket.ServerpacketHandlers")
		return nil, err
	}

	packetHandler.Append(lineschemaPacketHandlers...)
	packetHandler.Append(packet.NewJsonMergeInputPacket())                                              //增加合并输入数据
	namespaceInput := fmt.Sprintf("%s%s", setting.Api.ApiName, pathtransfer.Transfer_Direction_input)   // 补充命名空间
	namespaceOutput := fmt.Sprintf("%s%s", setting.Api.ApiName, pathtransfer.Transfer_Direction_output) // 去除命名空间
	inputTransfer := setting.Api.InputPathTransfers.ModifySrcPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(strings.TrimPrefix(path, namespaceInput), ".")
		return newPath
	}).GjsonPath()
	outputTransfer := setting.Api.OutputPathTransfers.Reverse().ModifyDstPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(strings.TrimPrefix(path, namespaceOutput), ".")
		return newPath
	}).GjsonPath()

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(inputTransfer, outputTransfer)
	packetHandler.Append(transferHandler)
	//生成注入函数
	injectObject := InjectObject{}
	injectObject.ExecSQLTPL = func(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
		return capi.ExecSQLTPL(ctx, tplName, input)
	}
	injectObject.PathTransfers = setting.Torms.Transfers()

	capi._api.Flow.DropEmpty()

	project := setting.Project
	capi._api._Project = project
	for language, scripts := range project.Scripts.GroupByLanguage() {
		engine, err := setting.Project.ScriptEngines.GetByLanguage(language) // 优先使用项目配置
		if errors.Is(err, goscript.ERROR_NOT_FOUND_SCRIPTI_BY_LANGUAGE) {
			engine, err = goscript.NewScriptEngine(language)
			if err != nil {
				return nil, err
			}
		}
		for _, script := range scripts {
			engine.WriteCode(script.Code)
		}
		callScript, err := project.FuncTransfers.GetCallFnScript(language)
		if err != nil {
			return nil, err
		}
		engine.WriteCode(callScript)
		capi._api._ScriptEngines.Add(engine)
	}
	return capi, nil
}

// Run 执行API
func (capi *apiCompiled) Run(ctx context.Context, inputJson string) (out string, err error) {

	outB, err := capi._api._apiStream.Run(ctx, []byte(inputJson))
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

// ExecSQLTPL 执行SQL语句
func (capi *apiCompiled) ExecSQLTPL(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
	tormIm, err := capi._Torms.GetByName(tplName)
	if err != nil {
		return nil, err
	}

	inputPathTransfers, outputPathTransfers := tormIm.Transfers.GetByNamespace(tormIm.Name).SplitInOut()
	funcName, err := pathtransfer.GetTransferFuncname(capi._api._Project.FuncTransfers, string(input), inputPathTransfers.GetAllDst())
	if err != nil {
		return nil, err
	}
	if funcName != "" {
		scriptEngine, err := capi._api._ScriptEngines.GetByLanguage(capi._api._Project.CurrentLanguage)
		if err != nil {
			return nil, err
		}
		input, err = TransferByFunc(capi._api._Project.FuncTransfers, scriptEngine, funcName, input) // 通过动态脚本修改入参
		if err != nil {
			return nil, err
		}
	}

	allPacketHandlers := make(packethandler.PacketHandlers, 0)
	namespaceInput := fmt.Sprintf("%s%s", tormIm.Name, pathtransfer.Transfer_Direction_input)   //去除命名空间
	namespaceOutput := fmt.Sprintf("%s%s", tormIm.Name, pathtransfer.Transfer_Direction_output) // 补充命名空间
	inputGopath := inputPathTransfers.Reverse().ModifyDstPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(path, namespaceInput)
		return newPath
	}).GjsonPath()
	outputGopath := outputPathTransfers.ModifySrcPath(func(path string) (newPath string) {
		newPath = strings.TrimPrefix(path, namespaceOutput)
		return newPath
	}).GjsonPath()
	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(inputGopath, outputGopath)
	allPacketHandlers.Append(transferHandler)
	allPacketHandlers.Append(packet.NewTormPackHandler(*tormIm))

	prov := tormIm.Source.Provider
	dbProvider, ok := prov.(*sqlexec.ExecutorSQL)
	if !ok {
		err = errors.Errorf("ExecSQLTPL required sourceprovider.DBProvider source,got:%s", prov.TypeName())
		return nil, err
	}
	db := dbProvider.GetDB()
	if db == nil {
		err = errors.Errorf("ExecSQLTPL sourceprovider.DBProvider.GetDB required,got nil (%s)", prov.TypeName())
		return nil, err
	}
	databaseName, err := sqlexec.GetDatabaseName(db)
	if err != nil {
		return nil, err
	}
	cudeventPack := packet.NewCUDEventPackHandler(db, databaseName)
	allPacketHandlers.Append(cudeventPack)
	mysqlPack := packet.NewMysqlPacketHandler(db)
	allPacketHandlers.Append(mysqlPack)
	packetHandlers, err := allPacketHandlers.GetByName(tormIm.Flow...)
	if err != nil {
		return nil, err
	}
	s := stream.NewStream(tplName, nil, packetHandlers...)
	out, err = s.Run(ctx, []byte(input))
	if err != nil {
		return nil, err
	}
	return out, nil
}
