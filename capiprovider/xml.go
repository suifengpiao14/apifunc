package xmlprovider

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/apifunc"
	"github.com/suifengpiao14/glob"
	"github.com/suifengpiao14/pathtransfer"
)

type xmlApiTable struct {
	XMLName xml.Name    `xml:"RECORDS"`
	Records []ApiRecord `xml:"RECORD"`
}
type xmlSourceTable struct {
	XMLName xml.Name       `xml:"RECORDS"`
	Records []SourceRecord `xml:"RECORD"`
}
type xmlTemplateTable struct {
	XMLName xml.Name         `xml:"RECORDS"`
	Records []TemplateRecord `xml:"RECORD"`
}

type ApiRecord struct {
	ApiID            string `xml:"api_id"`
	Title            string `xml:"title"`
	Method           string `xml:"method"`
	Route            string `xml:"route"`
	Script           string `xml:"script"`
	Dependents       string `xml:"dependents"`
	InputSchema      string `xml:"input_schema"`
	OutputSchema     string `xml:"output_schema"`
	PathTransferLine string `xml:"pathTransfers"`
	Flows            string `xml:"flows"`
}
type ApiRecords []ApiRecord

func (as ApiRecords) GetByRoute(route string, method string) (out ApiRecord, ok bool) {
	for _, a := range as {
		if strings.EqualFold(a.Method, method) && a.Route == route {
			return a, true
		}
	}
	return out, false
}

type SourceRecord struct {
	SourceID   string `xml:"source_id"`
	ENV        string `xml:"env"`
	SourceType string `xml:"source_type"`
	Config     string `xml:"config"`
	DDL        string `xml:"ddl"` //SQL 类型，需要使用cudevent 库时需要配置DDL
}

type SourceRecords []SourceRecord

func (ss SourceRecords) FilterByEnv(env string) (out SourceRecords) {
	out = make(SourceRecords, 0)
	for _, s := range ss {
		if s.ENV == env {
			out = append(out, s)
		}
	}
	return out
}

type TemplateRecord struct {
	TemplateID       string `xml:"template_id"`
	Title            string `xml:"title"`
	SourceID         string `xml:"source_id"`
	Tpl              string `xml:"tpl"`
	Type             string `xml:"type"`
	PathTransferLine string `xml:"pathTransfers"`
	Flows            string `xml:"flows"`
}

type TemplateRecords []TemplateRecord

func (ts TemplateRecords) GetAllByIds(ids ...string) (out TemplateRecords) {
	out = make(TemplateRecords, 0)
	for _, t := range ts {
		for _, id := range ids {
			if t.TemplateID == id {
				out = append(out, t)
			}
		}
	}
	return out
}

func loadDataFromFile(rootDir string, patten string) (out [][]byte, err error) {
	patten = filepath.Join(rootDir, patten)
	allFileList, err := glob.GlobDirectory(patten)
	if err != nil {
		err = errors.WithStack(err)
		return nil, err
	}
	out = make([][]byte, 0)
	for _, filename := range allFileList {
		b, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func LoadXmlDB(apiFileDir string, sourceFileDir string, tormFileDir string) (dbApiRecords ApiRecords, dbSourceRecords SourceRecords, dbTemplateRecords TemplateRecords, err error) {
	apiRecordAllFile, err := loadDataFromFile(apiFileDir, "**")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, apiReordOneFile := range apiRecordAllFile {
		reader := bytes.NewReader(apiReordOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlApiTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, err
		}
		dbApiRecords = append(dbApiRecords, table.Records...)

	}

	sourceAllFile, err := loadDataFromFile(sourceFileDir, "**")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, sourceOneFile := range sourceAllFile {
		reader := bytes.NewReader(sourceOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlSourceTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, err
		}
		dbSourceRecords = append(dbSourceRecords, table.Records...)
	}
	templateAllFile, err := loadDataFromFile(tormFileDir, "**")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, templateOneFile := range templateAllFile {
		reader := bytes.NewReader(templateOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlTemplateTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, err
		}
		dbTemplateRecords = append(dbTemplateRecords, table.Records...)
	}
	return dbApiRecords, dbSourceRecords, dbTemplateRecords, nil
}

func ConvertType(dbApiRecords ApiRecords, dbSourceRecords SourceRecords, dbTemplateRecords TemplateRecords) (apiModels apifunc.ApiModels, sourceModels apifunc.SourceModels, tormModels apifunc.TormModels) {

	apiModels, sourceModels, tormModels = make(apifunc.ApiModels, 0), make(apifunc.SourceModels, 0), make(apifunc.TormModels, 0)
	for _, apiRecord := range dbApiRecords {
		apiModel := apifunc.ApiModel{
			ApiId:            apiRecord.ApiID,
			Title:            apiRecord.Title,
			Method:           apiRecord.Method,
			Route:            apiRecord.Route,
			Script:           apiRecord.Script,
			Dependents:       apifunc.DependentJson(apiRecord.Dependents),
			InputSchema:      apiRecord.InputSchema,
			OutputSchema:     apiRecord.OutputSchema,
			PathTransferLine: pathtransfer.TransferLine(apiRecord.PathTransferLine),
			Flows:            apiRecord.Flows,
		}
		apiModels = append(apiModels, apiModel)
	}
	for _, sourceRecord := range dbSourceRecords {
		sourceModel := apifunc.SourceModel{
			SourceID:   sourceRecord.SourceID,
			ENV:        sourceRecord.ENV,
			SourceType: sourceRecord.SourceType,
			Config:     sourceRecord.Config,
			DDL:        sourceRecord.DDL,
		}
		sourceModels = append(sourceModels, sourceModel)
	}
	for _, sourceRecord := range dbTemplateRecords {
		sourceModel := apifunc.TormModel{
			TemplateID:       sourceRecord.TemplateID,
			Title:            sourceRecord.Title,
			SourceID:         sourceRecord.SourceID,
			Tpl:              sourceRecord.Tpl,
			Type:             sourceRecord.Type,
			PathTransferLine: pathtransfer.TransferLine(sourceRecord.PathTransferLine),
			Flows:            sourceRecord.Flows,
		}
		tormModels = append(tormModels, sourceModel)
	}
	return apiModels, sourceModels, tormModels

}
