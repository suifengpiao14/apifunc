package apifunc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/apifunc/apifunctemplate"
	"github.com/suifengpiao14/apifunc/provider"
	"github.com/suifengpiao14/lineschema"
	"github.com/suifengpiao14/stream"
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
	"github.com/suifengpiao14/torm"
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
	ExecSQLTPL ExecSouceFn
}

type DynamicLogicFn func(ctx context.Context, InjectObject InjectObject, input []byte) (out []byte, err error)

// Setting 配置
type Setting struct {
	Method                 string                  `json:"method"`
	Route                  string                  `json:"route"` // 路由,唯一
	RequestLineschema      string                  `json:"requestLineschema"`
	ResponseLineschema     string                  `json:"responseLineschema"`
	TemplateSourceSettings []TemplateSourceSetting `json:"templateSourceSetting"` // 模板、资源配置（执行模板后调用什么资源）这里存在模板名称和资源绑定关系，需要在配置中提现这种关系
	InputTransferPath      string                  `json:"inputConvertPath"`      // 输入数据转换器(由http请求数据转换为serviceFn中需要的格式,减少ServiceFn 中体力劳动)
	OutputTransferPath     string                  `json:"outputConvertPath"`     // 输出数据转换器(由ServiceFn输出数据转换为http响应数据,减少ServiceFn 中体力劳动)
	BusinessLogicFn        DynamicLogicFn
	errorHandler           stream.ErrorHandler
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

// TemplateSourceSetting 模板和执行器之间存在确定关系，在配置中体现, 同一个TemplateSourceSetting 下template 内的define 共用相同资源
type TemplateSourceSetting struct {
	Source    Source `json:"source"`
	Templates string `json:"templates"`
}

type apiCompiled struct {
	context           context.Context
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
	template          *apifunctemplate.ApifuncTemplate
	_container        *Container

	_stream     *stream.Stream //api stream
	_tormStream *stream.Stream // sql stream
}

func NewApiCompiled(api *Setting) (capi *apiCompiled, err error) {
	apiName := fmt.Sprintf("%s_%s", api.Method, api.Route)
	capi = &apiCompiled{
		Method:     api.Method,
		Route:      api.Route,
		sourcePool: NewSourcePool(),
		template: &apifunctemplate.ApifuncTemplate{
			Template: apifunctemplate.NewTemplate(),
		},
		_stream:           stream.NewStream(apiName, api.errorHandler),
		_tormStream:       stream.NewStream(fmt.Sprintf("torm_%s", apiName), nil),
		inputConvertPath:  api.InputTransferPath,
		outputConvertPath: api.OutputTransferPath,
	}

	for _, templateSourceSetting := range api.TemplateSourceSettings {
		//注册模板
		t := apifunctemplate.NewTemplate()
		t, err := t.Parse(templateSourceSetting.Templates)
		if err != nil {
			return nil, err
		}
		capi.template.Template, err = capi.template.Template.AddParseTree(t.ParseName, t.Tree)
		if err != nil {
			return nil, err
		}

		capi.template.TPL = strings.TrimSpace(fmt.Sprintf("%s\n%s", capi.template.TPL, templateSourceSetting.Templates))

		// 注册资源
		source := templateSourceSetting.Source
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
		//注册模板资源关联关系
		definedNames := strings.Split(strings.TrimPrefix(t.DefinedTemplates(), "; defined templates are: "), ",") //"; defined templates are: " 为固定字符串
		err = capi.setTemplateDependSource(source.Identifer, definedNames...)
		if err != nil {
			return nil, err
		}
	}

	packetHandler := stream.NewPacketHandlers()
	//验证、格式化 入参
	lineschemaPacketHandlers, err := lineschemapacket.ServerpacketHandlers(api)
	if err != nil {
		return nil, err
	}
	packetHandler.Append(lineschemaPacketHandlers...)
	//转换为代码中期望的数据格式
	transferHandler := packet.NewTransferPacketHandler(api.InputTransferPath, api.OutputTransferPath)
	packetHandler.Append(transferHandler)
	//生成注入函数
	injectObject := InjectObject{}
	injectObject.ExecSQLTPL = func(ctx context.Context, tplName string, input []byte) (out []byte, err error) {
		return capi.ExecSQLTPL(ctx, tplName, input)
	}
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
	sqlStr, _, _, err := torm.GetSQLFromTemplate(capi.template.Template, tplName, &volume)
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
