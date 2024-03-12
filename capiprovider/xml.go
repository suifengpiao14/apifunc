package capiprovider

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

type xmlDictionaryTable struct {
	XMLName xml.Name             `xml:"RECORDS"`
	Records []TransferFuncRecord `xml:"RECORD"`
}
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

type TransferFuncRecord struct {
	Language     string `xml:"language"`
	Script       string `xml:"script"`
	TransferLine string `xml:"transfer_line"`
}

type TransferFuncRecords []TransferFuncRecord

type ApiRecord struct {
	ApiID        string `xml:"api_id"`
	Title        string `xml:"title"`
	Method       string `xml:"method"`
	Route        string `xml:"route"`
	Script       string `xml:"script"`
	Dependents   string `xml:"dependents"`
	InputSchema  string `xml:"input_schema"`
	OutputSchema string `xml:"output_schema"`
	TransferLine string `xml:"transfer_line"`
	Flow         string `xml:"flow"`
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
	SSHConfig  string `xml:"ssh_config"`
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
	TemplateID    string `xml:"template_id"`
	SubTemplateID string `xml:"sub_template_id"`
	Title         string `xml:"title"`
	SourceID      string `xml:"source_id"`
	Tpl           string `xml:"tpl"`
	Type          string `xml:"type"`
	TransferLine  string `xml:"transfer_line"`
	Flow          string `xml:"flow"`
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

//LoadXmlDB 从XML中加载数据
func LoadXmlDB(env string, dictFileDir string, apiFileDir string, sourceFileDir string, tormFileDir string) (transferFuncModels apifunc.TransferFuncModels, apiModels apifunc.ApiModels, sourceModels apifunc.SourceModels, tormModels apifunc.TormModels, err error) {
	transferFuncRecords, apiRecords, sourceRecords, templateRecords := make(TransferFuncRecords, 0), make(ApiRecords, 0), make(SourceRecords, 0), make(TemplateRecords, 0)

	dictRecordAllFile, err := loadDataFromFile(dictFileDir, "**/*.xml")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, dicReordOneFile := range dictRecordAllFile {
		reader := bytes.NewReader(dicReordOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlDictionaryTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		transferFuncRecords = append(transferFuncRecords, table.Records...)
	}

	apiRecordAllFile, err := loadDataFromFile(apiFileDir, "**/*.xml")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, apiReordOneFile := range apiRecordAllFile {
		reader := bytes.NewReader(apiReordOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlApiTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		apiRecords = append(apiRecords, table.Records...)

	}

	sourceAllFile, err := loadDataFromFile(sourceFileDir, "**/*.xml")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, sourceOneFile := range sourceAllFile {
		reader := bytes.NewReader(sourceOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlSourceTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		sourceRecords = append(sourceRecords, table.Records...)
	}

	sourceRecords = sourceRecords.FilterByEnv(env)
	templateAllFile, err := loadDataFromFile(tormFileDir, "**/*.xml")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, templateOneFile := range templateAllFile {
		reader := bytes.NewReader(templateOneFile)
		decodeXML := xml.NewDecoder(reader)
		decodeXML.Strict = false
		table := xmlTemplateTable{}
		err = decodeXML.Decode(&table)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		templateRecords = append(templateRecords, table.Records...)
	}

	transferFuncModels, apiModels, sourceModels, tormModels = convertToModel(transferFuncRecords, apiRecords, sourceRecords, templateRecords)
	return transferFuncModels, apiModels, sourceModels, tormModels, nil
}

func convertToModel(transferFuncRecords TransferFuncRecords, dbApiRecords ApiRecords, dbSourceRecords SourceRecords, dbTemplateRecords TemplateRecords) (transferFuncModels apifunc.TransferFuncModels, apiModels apifunc.ApiModels, sourceModels apifunc.SourceModels, tormModels apifunc.TormModels) {

	transferFuncModels, apiModels, sourceModels, tormModels = make(apifunc.TransferFuncModels, 0), make(apifunc.ApiModels, 0), make(apifunc.SourceModels, 0), make(apifunc.TormModels, 0)

	for _, transferFuncRecord := range transferFuncRecords {
		transferFuncModel := apifunc.TransferFuncModel{
			Language:     transferFuncRecord.Language,
			Script:       transferFuncRecord.Script,
			TransferLine: pathtransfer.TransferLine(transferFuncRecord.TransferLine),
		}
		transferFuncModels = append(transferFuncModels, transferFuncModel)
	}

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
			PathTransferLine: pathtransfer.TransferLine(apiRecord.TransferLine),
			Flow:             apiRecord.Flow,
		}
		apiModels = append(apiModels, apiModel)
	}
	for _, sourceRecord := range dbSourceRecords {
		sourceModel := apifunc.SourceModel{
			SourceID:   sourceRecord.SourceID,
			ENV:        sourceRecord.ENV,
			SourceType: sourceRecord.SourceType,
			Config:     sourceRecord.Config,
			SSHConfig:  sourceRecord.SSHConfig,
			DDL:        sourceRecord.DDL,
		}
		sourceModels = append(sourceModels, sourceModel)
	}
	for _, templateRecord := range dbTemplateRecords {
		tormModel := apifunc.TormModel{
			TemplateID:   templateRecord.TemplateID,
			Title:        templateRecord.Title,
			SourceID:     templateRecord.SourceID,
			Tpl:          templateRecord.Tpl,
			Type:         templateRecord.Type,
			TransferLine: pathtransfer.TransferLine(templateRecord.TransferLine),
			Flow:         templateRecord.Flow,
		}
		tormModels = append(tormModels, tormModel)
	}
	return transferFuncModels, apiModels, sourceModels, tormModels

}
