package apifunc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/suifengpiao14/apifunc"
	"github.com/suifengpiao14/logchan/v2"
)

func TestAPIMemory(t *testing.T) {
	route := "/api/1/hello"
	method := "POST"
	tplStr := `
	{{define "PaginateWhere"}} {{end}}
	{{define "PaginateTotal"}} select count(*) as count from component where 1=1 {{template "PaginateWhere" .}} and deleted_at is null; {{end}}
	{{define "Paginate"}} select * from component where 1=1 {{template "PaginateWhere" .}} and deleted_at is null order by updated_at desc limit :Offset,:Limit ; {{end}}
	`
	sourceIdentifer := "test_provider"
	sourceConfig := `
	   	{"logLevel":"debug","dsn":"root:123456@tcp(10.0.11.125:3306)/office_web_site?charset=utf8&timeout=1s&readTimeout=5s&writeTimeout=5s&parseTime=False&loc=Local&multiStatements=true","timeout":30}
	   	`
	MemoryDB := apifunc.MemoryDB{
		InOutMap: map[string]string{},
	}
	selectSQL := "select * from component where 1=1   and deleted_at is null order by updated_at desc limit 0,20 ;"
	countSQL := "select count(*) as count from component where 1=1   and deleted_at is null;"
	MemoryDB.InOutMap[selectSQL] = "[{\"content\":\"\",\"created_at\":\"2022-11-17 22:03:11\",\"deleted_at\":\"\",\"description\":\"支持微信、支付宝接入,封装了异常重试、对账等功能,拿来即可使用\",\"icon\":\"flaticon-research-1\",\"id\":\"6\",\"key\":\"pay\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another6.jpg\",\"title\":\"支付组件\",\"updated_at\":\"2022-11-17 22:03:23\"},{\"content\":\"\",\"created_at\":\"2022-11-17 22:01:43\",\"deleted_at\":\"\",\"description\":\"状态是实体常有的重要属性之一,有限状态机巧妙的化解了状态变迁的难点,为程序健壮提供有力保障\",\"icon\":\"flaticon-branding\",\"id\":\"5\",\"key\":\"fsm\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another5.jpg\",\"title\":\"有限状态机\",\"updated_at\":\"2022-11-17 22:02:05\"},{\"content\":\"\",\"created_at\":\"2022-11-17 21:59:47\",\"deleted_at\":\"\",\"description\":\"接口参数校验,是频繁而无技术挑战的必备工作,经过大量总结后,采用合适的标准,构造通用参数校验组件,极大提升接口的内聚性和一致性,并将其交给机器员工,极大的减少了人为工作量\",\"icon\":\"flaticon-education\",\"id\":\"4\",\"key\":\"validator\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another4.jpg\",\"title\":\"参数校验组件\",\"updated_at\":\"2022-11-17 22:00:44\"},{\"content\":\"\",\"created_at\":\"2022-11-17 21:58:58\",\"deleted_at\":\"\",\"description\":\"excel导出功能非常常见,看似导出数据各异,经过深入思考后抽离共性,实现稳定通用的导出组件\",\"icon\":\"flaticon-web-programming\",\"id\":\"3\",\"key\":\"excel\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another3.jpg\",\"title\":\"excel导出组件\",\"updated_at\":\"2022-11-17 21:59:15\"},{\"content\":\"\",\"created_at\":\"2022-11-17 21:53:42\",\"deleted_at\":\"\",\"description\":\"账户组件包含用户登录、注册、认证、找回密码、鉴权、强安全校验等基础功能.支持auth2.0,openID等国际标准\",\"icon\":\"flaticon-research-1\",\"id\":\"2\",\"key\":\"account\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another2.jpg\",\"title\":\"账户组件\",\"updated_at\":\"2022-11-17 21:54:42\"},{\"content\":\"\",\"created_at\":\"2022-11-17 21:51:27\",\"deleted_at\":\"\",\"description\":\"地址库包含省市区数据维护和应用,用户快递收货地址,用户可以是人、商户或者公司\",\"icon\":\"flaticon-branding\",\"id\":\"1\",\"key\":\"address\",\"label\":\"\",\"thumb\":\"../assets/images/case-studies/case-studies-another1.jpg\",\"title\":\"地址库\",\"updated_at\":\"2022-11-17 21:53:08\"}]"
	MemoryDB.InOutMap[countSQL] = "6"

	api := &apifunc.Setting{
		Method: "post,get",
		Route:  route,
		RequestLineschema: `version=http://json-schema.org/draft-07/schema,id=input,direction=in
		fullname=pageIndex,dst=pageIndex,format=number,required
		fullname=pageSize,dst=pageSize,format=number,required`,
		ResponseLineschema: `version=http://json-schema.org/draft-07/schema,id=output,direction=out
		fullname=items[].content,src=PaginateOut.#.content,required
		fullname=items[].createdAt,src=PaginateOut.#.created_at,required
		fullname=items[].deletedAt,src=PaginateOut.#.deleted_at,required
		fullname=items[].description,src=PaginateOut.#.description,required
		fullname=items[].icon,src=PaginateOut.#.icon,required
		fullname=items[].id,src=PaginateOut.#.id,required
		fullname=items[].key,src=PaginateOut.#.key,required
		fullname=items[].label,src=PaginateOut.#.label,required
		fullname=items[].thumb,src=PaginateOut.#.thumb,required
		fullname=items[].title,src=PaginateOut.#.title,required
		fullname=items[].updatedAt,src=PaginateOut.#.updated_at,required
		fullname=pageInfo.pageIndex,src=input.pageIndex,required
		fullname=pageInfo.pageSize,src=input.pageSize,required
		fullname=pageInfo.total,src=PaginateTotalOut,required`,
		//{pageInfo:{pageIndex:input.pageIndex,pageSize:input.pageSize,total:PaginateTotalOut},items:{content:PaginateOut.#.content,createdAt:PaginateOut.#.created_at,deletedAt:PaginateOut.#.deleted_at}|@group}
		/* 		PreScript: `
		   		input:=storage.GetMemory()
		   		input["Limit"]=int(input["pageSize"])
		   		input["Offset"]=int(input["pageIndex"]*input["pageSize"])
		   		`,
		   		MainScript: `
		   		input:=storage.GetMemory()
		   		ctx:=storage.GetCtx()
		   		PaginateTotalOut:=execSQLTPL(ctx,"PaginateTotal",input)
		   		storage.SetRaw("PaginateTotalOut",PaginateTotalOut)
		   		PaginateOut :="[]"
		   		if PaginateTotalOut {
		   		PaginateOut =execSQLTPL(ctx,"Paginate",input)
		   		}
		   		storage.SetRaw("PaginateOut",PaginateOut)
		   		`,
		   		PostScript: ``,
		   		AfterEvent: ``, */
		Torms: []apifunc.Torm{
			{
				Tpl: tplStr,
				Source: apifunc.Source{
					Identifer: sourceIdentifer,
					Type:      apifunc.PROVIDER_SQL,
					Config:    sourceConfig,
					Provider:  &MemoryDB,
				},
			},
		},
	}
	ctx := context.Background()
	capi, err := apifunc.NewApiCompiled(api)
	if err != nil {
		panic(err)
	}

	container := apifunc.NewContainer(func(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error) {
		fmt.Println(logInfo)
	})
	container.RegisterAPI(capi)

	routeCapi, err := container.GetCApi(route, method)
	if err != nil {
		panic(err)
	}

	inputJson := `{"pageIndex":"0","pageSize":"20"}`
	ctx = context.WithValue(ctx, "traceID", "12345")
	out, err := routeCapi.Run(ctx, inputJson)
	if err != nil {
		panic(err)
	}
	fmt.Println(out)

}
