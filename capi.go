package apifunc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/apifunc/apifunctemplate"
	"github.com/suifengpiao14/apifunc/provider"
	"github.com/suifengpiao14/lineschema"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
	"github.com/suifengpiao14/torm"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/xeipuuv/gojsonschema"
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
	ExecSQLTPL   ExecSouceFn
	vocabularies Vocabularies // 记录vocabulary
}

func (injectObject InjectObject) ConvertInput(namespace string, input []byte) (out []byte) {
	vocabularies := injectObject.vocabularies.GetByParent(namespace)
	vocabularies.FilterEmpty()
	gjsonpath := vocabularies.Transfers().Reverse().String()
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
	ApiName            string `json:"apiName"` // 接口名称
	Method             string `json:"method"`
	Route              string `json:"route"` // 路由,唯一
	RequestLineschema  string `json:"requestLineschema"`
	ResponseLineschema string `json:"responseLineschema"`
	Torms              Torms  `json:"torms"`             // 模板、资源配置（执行模板后调用什么资源）这里存在模板名称和资源绑定关系，需要在配置中提现这种关系
	InputTransferPath  string `json:"inputConvertPath"`  // 输入数据转换器(由http请求数据转换为serviceFn中需要的格式,减少ServiceFn 中体力劳动)
	OutputTransferPath string `json:"outputConvertPath"` // 输出数据转换器(由ServiceFn输出数据转换为http响应数据,减少ServiceFn 中体力劳动)
	BusinessLogicFn    DynamicLogicFn
	errorHandler       stream.ErrorHandler
}

func (s Setting) GetRoute() (mehtod string, path string) {
	return s.Method, s.Route
}
func (s Setting) UnpackSchema() (lineschema string) {
	return s.RequestLineschema
}
func (s Setting) PackSchema() (lineschema string) {
	return s.ResponseLineschema
}

// Torm 模板和执行器之间存在确定关系，在配置中体现, 同一个Torm 下template 内的define 共用相同资源
type Torm struct {
	Name               string       `json:"name"`
	Source             Source       `json:"source"`
	Tpl                string       `json:"tpl"`
	InputVocabularies  Vocabularies `json:"inputVocabularies"`
	OutputVocabularies Vocabularies `json:"outputVocabularies"`
}

type Torms []Torm

func (ts *Torms) Add(subTorms ...Torm) {
	if *ts == nil {
		*ts = make(Torms, 0)
	}
	*ts = append(*ts, subTorms...)
}
func (ts *Torms) Vocabularies() (vocabularies Vocabularies) {
	vocabularies = make(Vocabularies, 0)
	for _, t := range *ts {
		vocabularies.AddReplace(t.InputVocabularies...)
		vocabularies.AddReplace(t.OutputVocabularies...)
	}

	return vocabularies
}

// NamespaceInput 从标准库转换过来的命名空间
func (t Torm) NamespaceInput() (namespaceInput string) {
	return fmt.Sprintf("%s.input", t.Name)
}

// NamespaceInput 返回到标准库时，增加的命名空间
func (t Torm) NamespaceOutput() (namespaceOutput string) {
	return fmt.Sprintf("%s.output", t.Name)
}

//FormatInput 从标准输入中获取数据
func (t Torm) FormatInput(data []byte) (input string) {
	gjsonPath := t.InputVocabularies.Transfers().Reverse().String()
	input = gjson.GetBytes(data, gjsonPath).String()
	input = gjson.Get(input, t.NamespaceInput()).String()
	return input
}

//FormatOutput 格式化输出标准数据
func (t Torm) FormatOutput(data []byte) (output []byte) {
	gjsonPath := t.OutputVocabularies.Transfers().String()
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
	context           context.Context
	ApiName           string `json:"apiName"`
	Route             string `json:"route"`
	Method            string
	inputDefaultJson  string
	inputSchema       *gojsonschema.JSONLoader
	inputConvertPath  string
	inputLineSchema   *lineschema.Lineschema
	outputDefaultJson string
	outputSchema      *gojsonschema.JSONLoader
	outputConvertPath string
	outputLineSchema  *lineschema.Lineschema
	BusinessLogicFn   DynamicLogicFn // 业务处理函数
	sourcePool        *SourcePool
	template          *template.Template
	_container        *Container

	_stream     *stream.Stream //api stream
	_tormStream *stream.Stream // sql stream
}

var ERROR_COMPILED_API = errors.New("compiled api error")

func NewApiCompiled(api *Setting) (capi *apiCompiled, err error) {
	defer func() {
		if err != nil {
			err = errors.WithMessage(err, ERROR_COMPILED_API.Error())
		}
	}()
	apiName := fmt.Sprintf("%s_%s", api.Method, api.Route)
	capi = &apiCompiled{
		ApiName:           api.ApiName,
		Method:            api.Method,
		Route:             api.Route,
		sourcePool:        NewSourcePool(),
		template:          apifunctemplate.NewTemplate(),
		_stream:           stream.NewStream(apiName, api.errorHandler),
		_tormStream:       stream.NewStream(fmt.Sprintf("torm_%s", apiName), nil),
		inputConvertPath:  api.InputTransferPath,
		outputConvertPath: api.OutputTransferPath,
	}

	allTplArr := make([]string, 0)

	// 收集模板，注册资源，模板名称关联关系
	for _, torm := range api.Torms {
		//注册模板
		t := apifunctemplate.NewTemplate()
		t, err := t.Parse(torm.Tpl)
		if err != nil {
			return nil, err
		}
		allTplArr = append(allTplArr, torm.Tpl)

		// 注册资源
		source := torm.Source
		if source.Provider == nil {
			source, err = MakeSource(source.Identifer, source.Type, source.Config)
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

	err = lineschemapacket.RegisterLineschemaPacket(api)
	if err != nil {
		return nil, err
	}

	packetHandler := stream.NewPacketHandlers()
	//验证、格式化 入参
	lineschemaPacketHandlers, err := lineschemapacket.ServerpacketHandlers(api)
	if err != nil {
		err = errors.WithMessage(err, "lineschemapacket.ServerpacketHandlers")
		return nil, err
	}
	packetHandler.Append(lineschemaPacketHandlers...)
	namespaceAdd := fmt.Sprintf("%s.input", api.ApiName)   // 补充命名空间
	namespaceTrim := fmt.Sprintf("%s.output", api.ApiName) // 去除命名空间
	packetHandler.Append(packet.NewJsonAddTrimNamespacePacket(namespaceAdd, namespaceTrim))

	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(api.InputTransferPath, api.OutputTransferPath)
	packetHandler.Append(transferHandler)
	//生成注入函数
	injectObject := InjectObject{}
	injectObject.ExecSQLTPL = func(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
		return capi.ExecSQLTPL(ctx, tplName, input)
	}
	injectObject.vocabularies = api.Torms.Vocabularies()
	//注入逻辑处理函数
	if api.BusinessLogicFn != nil {
		packetHandler.Append(packet.NewFuncPacketHandler(func(ctx context.Context, input []byte) (newCtx context.Context, out []byte, err error) {
			out, err = api.BusinessLogicFn(ctx, injectObject, input)
			return ctx, out, err
		}, nil))
	}
	capi._stream.AddPack(packetHandler...)

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

// Run 执行API
func (capi *apiCompiled) Run(ctx context.Context, inputJson string) (out string, err error) {
	outB, err := capi._stream.Run(ctx, []byte(inputJson))
	if err != nil {
		return "", err
	}
	out = string(outB)
	return out, nil
}

// ExecSQLTPL 执行SQL语句
func (capi *apiCompiled) ExecSQLTPL(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
	volume := apifunctemplate.VolumeMap{}
	if len(input) > 0 {
		err = json.Unmarshal(input, &volume)
		if err != nil {
			return nil, err
		}
	}
	status, ok := volume["Status"]//HsbRemark
	if ok {
		volume["Status"] = int(status.(float64))
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
	s := stream.NewStream(tplName, nil)
	cudeventPack := packet.NewCUDEventPackHandler(db)
	s.AddPack(cudeventPack)
	mysqlPack := packet.NewMysqlPacketHandler(db)
	s.AddPack(mysqlPack)
	out, err = s.Run(ctx, []byte(sqlStr))
	if err != nil {
		return nil, err
	}
	return out, nil
}
