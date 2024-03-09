package apifunc

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/pkg/errors"
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
	packet.PACKETHANDLER_NAME_JsonAddTrimNamespacePacket,
	packet.PACKETHANDLER_NAME_TransferPacketHandler,
	PACKETHANDLER_NAME_API_LOGIC,
}

var DefaultTormFlows = packethandler.Flow{
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
}

func (injectObject InjectObject) ConvertInput(namespace string, input []byte) (out []byte) {
	pathTransfers := injectObject.PathTransfers.GetByNamespace(namespace)
	gjsonpath := pathTransfers.Reverse().GjsonPath()
	outStr := gjson.GetBytes(input, gjsonpath).String()
	if outStr != "" {
		outStr = gjson.Get(outStr, namespace).String()
	}
	out = []byte(outStr)
	return out
}

type DynamicLogicFn func(ctx context.Context, injectObject InjectObject, input []byte) (out []byte, err error)

// Setting 配置
type Setting struct {
	Api             Api        `json:"api"`
	Torms           torm.Torms `json:"torms"` // 模板、资源配置（执行模板后调用什么资源）这里存在模板名称和资源绑定关系，需要在配置中提现这种关系
	BusinessLogicFn DynamicLogicFn
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
	_apiStream          *stream.Stream
}

// api默认流程
var Api_Flow_Default = []string{}

func (s Api) GetRoute() (mehtod string, path string) {
	return s.Method, s.Route
}
func (s Api) UnpackSchema() (lineschema string) {
	return s.RequestLineschema
}
func (s Api) PackSchema() (lineschema string) {
	return s.ResponseLineschema
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
	namespaceAdd := fmt.Sprintf("%s%s", setting.Api.ApiName, pathtransfer.Transfer_Direction_input)   // 补充命名空间
	namespaceTrim := fmt.Sprintf("%s%s", setting.Api.ApiName, pathtransfer.Transfer_Direction_output) // 去除命名空间
	packetHandler.Append(packet.NewJsonAddTrimNamespacePacket(namespaceAdd, namespaceTrim))

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(setting.Api.InputPathTransfers.GjsonPath(), setting.Api.OutputPathTransfers.Reverse().GjsonPath())
	packetHandler.Append(transferHandler)
	//生成注入函数
	injectObject := InjectObject{}
	injectObject.ExecSQLTPL = func(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
		return capi.ExecSQLTPL(ctx, tplName, input)
	}
	injectObject.PathTransfers = setting.Torms.Transfers()
	//注入逻辑处理函数
	if setting.BusinessLogicFn != nil {
		packetHandler.Append(packet.NewFuncPacketHandler(PACKETHANDLER_NAME_API_LOGIC, func(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
			out, err = setting.BusinessLogicFn(ctx, injectObject, input)
			return ctx, out, err
		}, nil))
	}
	capi._api.Flow.DropEmpty()
	packetHandlers, err := packetHandler.GetByName(capi._api.Flow...)
	if err != nil {
		return nil, err
	}
	capi._api._apiStream = stream.NewStream(capi._api.ApiName, nil, packetHandlers...)
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
	volume := torm.VolumeMap{}
	if len(input) > 0 {
		m := make(map[string]any)
		err = json.Unmarshal(input, &m)
		if err != nil {
			return nil, err
		}
		convertFloatsToInt(m) // json.Unmarshal 出来后int 转为float64了，需要尝试优先使用int
		volume = torm.VolumeMap(m)
	}
	sqlStr, _, _, err := torm.GetSQLFromTemplate(capi.template, tplName, &volume)
	if err != nil {
		return nil, err
	}
	tormIm, err := capi._Torms.GetByName(tplName)
	if err != nil {
		return nil, err
	}
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

	allPacketHandlers := make(packethandler.PacketHandlers, 0)
	inputPathTransfers, outputPathTransfers := tormIm.Transfers.SplitInOut(tormIm.Name)

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(inputPathTransfers.Reverse().GjsonPath(), outputPathTransfers.GjsonPath())
	allPacketHandlers.Append(transferHandler)

	namespaceTrim := fmt.Sprintf("%s%s", tormIm.Name, pathtransfer.Transfer_Direction_input) //去除命名空间
	namespaceAdd := fmt.Sprintf("%s%s", tormIm.Name, pathtransfer.Transfer_Direction_output) // 补充命名空间
	allPacketHandlers.Append(packet.NewJsonTrimAddNamespacePacket(namespaceTrim, namespaceAdd))

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
	out, err = s.Run(ctx, []byte(sqlStr))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func convertFloatsToInt(data map[string]interface{}) {
	for key, value := range data {
		switch v := value.(type) {
		case float64:
			// 尝试将 float64 转换为 int
			if float64(int(v)) == v {
				data[key] = int(v)
			}
		case map[string]interface{}:
			// 递归处理嵌套的 map
			convertFloatsToInt(v)
		}
	}
}
