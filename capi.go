package apifunc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/apifunc/provider"
	"github.com/suifengpiao14/pathtransfer"
	"github.com/suifengpiao14/sqlexec"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
	"github.com/suifengpiao14/torm"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

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
	gjsonpath := pathTransfers.Reverse().String()
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
	Api             Api   `json:"api"`
	Torms           Torms `json:"torms"` // 模板、资源配置（执行模板后调用什么资源）这里存在模板名称和资源绑定关系，需要在配置中提现这种关系
	BusinessLogicFn DynamicLogicFn
	errorHandler    stream.ErrorHandler
}

type Api struct {
	ApiName             string                 `json:"apiName"`
	Route               string                 `json:"route"`
	Method              string                 `json:"method"`
	Flow                []string               `json:"flow"` // 按顺序组装执行流程
	BusinessLogicFn     DynamicLogicFn         // 业务处理函数
	RequestLineschema   string                 `json:"requestLineschema"`
	ResponseLineschema  string                 `json:"responseLineschema"`
	InputPathTransfers  pathtransfer.Transfers `json:"inputPathTransfers"`
	OutputPathTransfers pathtransfer.Transfers `json:"outPathTransfers"`
	_packetHandlers     stream.PacketHandlers
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

// Torm 模板和执行器之间存在确定关系，在配置中体现, 同一个Torm 下template 内的define 共用相同资源
type Torm struct {
	Name                string                 `json:"name"`
	Source              Source                 `json:"source"`
	Tpl                 string                 `json:"tpl"`
	InputPathTransfers  pathtransfer.Transfers `json:"inputPathTransfers"`
	OutputPathTransfers pathtransfer.Transfers `json:"outPathTransfers"`
	Flow                []string               `json:"flow"` // 按顺序组装执行流程
	_packetHandlers     stream.PacketHandlers  // 流程执行器组件集合
}

// torm默认流程
var Torm_Flow_Default = []string{}

type Torms []Torm

func (ts *Torms) Add(subTorms ...Torm) {
	if *ts == nil {
		*ts = make(Torms, 0)
	}
	*ts = append(*ts, subTorms...)
}
func (ts Torms) GetByName(name string) (t *Torm, err error) {
	for _, t := range ts {
		if strings.EqualFold(name, t.Name) {
			return &t, nil
		}
	}
	err = errors.Errorf("not found torm named:%s", name)
	return nil, err
}
func (ts *Torms) PathTransfers() (pathTransfers pathtransfer.Transfers) {
	pathTransfers = make(pathtransfer.Transfers, 0)
	for _, t := range *ts {
		pathTransfers.AddReplace(t.InputPathTransfers...)
		pathTransfers.AddReplace(t.OutputPathTransfers...)
	}

	return pathTransfers
}

// NamespaceInput 从标准库转换过来的命名空间
func (t Torm) NamespaceInput() (namespaceInput string) {
	return fmt.Sprintf("%s.input", t.Name)
}

// NamespaceInput 返回到标准库时，增加的命名空间
func (t Torm) NamespaceOutput() (namespaceOutput string) {
	return fmt.Sprintf("%s.output", t.Name)
}

// FormatInput 从标准输入中获取数据
func (t Torm) FormatInput(data []byte) (input string) {
	gjsonPath := t.InputPathTransfers.Reverse().String()
	input = gjson.GetBytes(data, gjsonPath).String()
	input = gjson.Get(input, t.NamespaceInput()).String()
	return input
}

// FormatOutput 格式化输出标准数据
func (t Torm) FormatOutput(data []byte) (output []byte) {
	gjsonPath := t.OutputPathTransfers.String()
	output = []byte(gjson.GetBytes(data, gjsonPath).String())
	output, err := sjson.SetRawBytes(output, t.NamespaceOutput(), output)
	if err != nil {
		panic(err)
	}
	return output
}

// GetTemplateNames 获取模板的define名称
func GetTemplateNames(t *template.Template) (tplNames []string) {
	tplNames = make([]string, 0)
	for _, tmp := range t.Templates() {
		tplNames = append(tplNames, tmp.Name())
	}

	return tplNames
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
	context       context.Context
	sourcePool    *SourcePool
	template      *template.Template
	_api          Api
	_container    *Container
	_Torms        Torms
	_tormStream   *stream.Stream // sql stream
	_errorHandler stream.ErrorHandler
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
	apiName := fmt.Sprintf("%s_%s", setting.Api.Method, setting.Api.Route)
	if len(setting.Api.Flow) == 0 {
		setting.Api.Flow = DefaultAPIFlows()
	}
	for i := range setting.Torms {
		if len(setting.Torms[i].Flow) == 0 {
			setting.Torms[i].Flow = DefaultTormFlows()
		}
	}
	capi = &apiCompiled{
		_api:          setting.Api,
		_Torms:        setting.Torms,
		sourcePool:    NewSourcePool(),
		template:      torm.NewTemplate(),
		_errorHandler: setting.errorHandler,
		_tormStream:   stream.NewStream(fmt.Sprintf("torm_%s", apiName), nil),
	}

	allTplArr := make([]string, 0)

	// 收集模板，注册资源，模板名称关联关系
	for _, orm := range setting.Torms {
		//注册模板
		t := torm.NewTemplate()
		t, err := t.Parse(orm.Tpl)
		if err != nil {
			return nil, err
		}
		allTplArr = append(allTplArr, orm.Tpl)

		// 注册资源
		source := orm.Source
		if source.Provider == nil {
			source, err = MakeSource(source.Identifer, source.Type, source.Config, source.DDL)
			if err != nil {
				return nil, err
			}
		}

		err = capi.sourcePool.RegisterSource(source)
		if err != nil {
			return nil, err
		}
		definedNames := GetTemplateNames(t)
		//注册模板资源关联关系
		err = capi.setTemplateDependSource(source.Identifer, definedNames...)
		if err != nil {
			return nil, err
		}
	}
	//注册模板
	allTplArs := strings.Join(allTplArr, "\n")
	capi.template, err = capi.template.Parse(allTplArs)
	if err != nil {
		return nil, err
	}

	err = lineschemapacket.RegisterLineschemaPacket(setting.Api)
	if err != nil {
		return nil, err
	}

	packetHandler := stream.NewPacketHandlers()
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
	transferHandler := packet.NewTransferPacketHandler(setting.Api.InputPathTransfers.String(), setting.Api.OutputPathTransfers.Reverse().String())
	packetHandler.Append(transferHandler)
	//生成注入函数
	injectObject := InjectObject{}
	injectObject.ExecSQLTPL = func(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
		return capi.ExecSQLTPL(ctx, tplName, input)
	}
	injectObject.PathTransfers = setting.Torms.PathTransfers()
	//注入逻辑处理函数
	if setting.BusinessLogicFn != nil {
		packetHandler.Append(packet.NewFuncPacketHandler(PACKETHANDLER_NAME_API_LOGIC, func(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
			out, err = setting.BusinessLogicFn(ctx, injectObject, input)
			return ctx, out, err
		}, nil))
	}
	capi._api._packetHandlers = packetHandler

	return capi, nil
}

// RelationTemplateAndSource 设置模版依赖的资源
func (capi *apiCompiled) setTemplateDependSource(sourceIdentifer string, templateIdentifers ...string) (err error) {
	for _, tplName := range templateIdentifers {
		tplName = strings.TrimSpace(tplName)
		err = capi.sourcePool.AddTemplateIdentiferRelation(tplName, sourceIdentifer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (capi *apiCompiled) Stream() (s *stream.Stream, err error) {
	packetHandlers, err := capi._api._packetHandlers.GetByName(capi._api.Flow...)
	if err != nil {
		return nil, err
	}
	s = stream.NewStream(fmt.Sprintf("%s_stream", capi._api.ApiName), capi._errorHandler, packetHandlers...)
	return s, nil
}

// Run 执行API
func (capi *apiCompiled) Run(ctx context.Context, inputJson string) (out string, err error) {
	streamImp, err := capi.Stream()
	if err != nil {
		return "", err
	}
	outB, err := streamImp.Run(ctx, []byte(inputJson))
	if err != nil {
		return "", err
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

	prov, err := capi.sourcePool.GetProviderByTemplateIdentifer(tplName)
	if err != nil {
		return nil, err
	}
	dbProvider, ok := prov.(*provider.DBProvider)
	if !ok {
		err = errors.Errorf("ExecSQLTPL required provider.DBProvider source,got:%s", prov.TypeName())
		return nil, err
	}
	db := dbProvider.GetDB()
	if db == nil {
		err = errors.Errorf("ExecSQLTPL provider.DBProvider.GetDB required,got nil (%s)", prov.TypeName())
		return nil, err
	}
	tormIm, err := capi._Torms.GetByName(tplName)
	if err != nil {
		return nil, err
	}

	allPacketHandlers := make(stream.PacketHandlers, 0)
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
