package apifunc

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/pathtransfer"
	"github.com/suifengpiao14/sqlexec"
	"github.com/suifengpiao14/sshmysql"
)

type DependentJson string

func (dj DependentJson) Dependents() (dependents Dependents, err error) {
	dependents = make(Dependents, 0)
	if dj == "" {
		return dependents, nil
	}
	err = json.Unmarshal([]byte(dj), &dependents)
	if err != nil {
		return nil, err
	}
	return dependents, nil
}

// Dependent 依赖关系
type Dependent struct {
	Fullname string `json:"fullname"`
	Type     string `json:"type"`
}

const (
	Dependent_Type_Torm = "torm"
)

type Dependents []Dependent

func (deps Dependents) FilterByType(typ string) (nDeps Dependents) {
	nDeps = make(Dependents, 0)
	for _, d := range deps {
		if strings.Contains(d.Type, typ) {
			nDeps = append(nDeps, d)
		}
	}
	return nDeps
}

func (deps Dependents) Fullnames() (fullnames []string) {
	fullnames = make([]string, 0)
	for _, d := range deps {
		fullnames = append(fullnames, d.Fullname)
	}
	return fullnames
}

//First 获取第一个
func (deps Dependents) First() (fullname string) {
	for _, d := range deps {
		return d.Fullname
	}
	return fullname
}

func (deps Dependents) String() (s string) {
	b, err := json.Marshal(deps)
	if err != nil {
		panic(err)
	}
	s = string(b)
	return s
}

type ApiModel struct {
	ApiId            string                    `json:"apiId"`
	Title            string                    `json:"title"`
	Method           string                    `json:"method"`
	Route            string                    `json:"route"`
	Script           string                    `json:"script"`
	Dependents       DependentJson             `json:"dependents"`
	InputSchema      string                    `json:"inputSchema"`
	OutputSchema     string                    `json:"outputSchema"`
	PathTransferLine pathtransfer.TransferLine `json:"pathTransfers"`
	Flow             string                    `json:"flow"`
}

type ApiModels []ApiModel

type TransferFuncModel struct {
	Language     string                    `xml:"language"`
	Script       string                    `xml:"script"`
	TransferLine pathtransfer.TransferLine `xml:"transfer_line"`
}

type TransferFuncModels []TransferFuncModel

type SourceModel struct {
	SourceID   string `json:"sourceId"`
	ENV        string `json:"env"`
	SourceType string `json:"sourceType"`
	Config     string `json:"config"`
	SSHConfig  string `json:"sshConfig"`
	DDL        string `json:"ddl"` //SQL 类型，需要使用cudevent 库时需要配置DDL
}

const (
	Source_Type_SQL  = "SQL"
	Source_Type_CURL = "CURL"
)

type SourceModels []SourceModel

//FillDDL 填充DDL
func (ss *SourceModels) FillDDL() (err error) {
	for i, sourceModel := range *ss {
		if sourceModel.DDL != "" {
			continue
		}
		switch strings.ToUpper(sourceModel.SourceType) {
		case Source_Type_SQL:
			c, err := sqlexec.JsonToDBConfig(sourceModel.Config)
			if err != nil {
				err = errors.WithMessagef(err, "sourceType:%s,config:%s", sourceModel.SourceType, sourceModel.Config)
				return err
			}
			var sshConfig *sshmysql.SSHConfig
			if sourceModel.SSHConfig != "" {
				sshConfig, err = sshmysql.JsonToSSHConfig(sourceModel.Config)
				if err != nil {
					err = errors.WithMessagef(err, "sshConfig:%s", sourceModel.SSHConfig)
					return err
				}
			}

			exec := sqlexec.NewExecutorSQL(*c, sshConfig)

			// 通过配置连接DB获取DDL
			ddl, err := sqlexec.GetDDL(exec.GetDB())
			if err != nil {
				err = errors.WithMessagef(err, "DSN:%s", c.DSN)
				return err
			}
			(*ss)[i].DDL = ddl
		}
	}
	return nil
}

type TormModel struct {
	TemplateID       string                    `json:"templateId"`
	SubTemplateNames []string                  `json:"SubTemplateNames"`
	Title            string                    `json:"title"`
	SourceID         string                    `json:"sourceId"`
	Tpl              string                    `json:"tpl"`
	Type             string                    `json:"type"`
	TransferLine     pathtransfer.TransferLine `json:"transferLine"`
	Flow             string                    `json:"flow"`
}

type TormModels []TormModel

func (tModels TormModels) GetByName(names ...string) (subModels TormModels) {
	subModels = make(TormModels, 0)
	for _, n := range names {
		for _, t := range tModels {
			if strings.EqualFold(t.TemplateID, n) {
				subModels = append(subModels, t)
				break
			}
		}
	}
	return subModels

}

func (tModels TormModels) GroupBySourceId() (out map[string]TormModels) {
	out = make(map[string]TormModels, 0)
	for _, t := range tModels {
		if _, ok := out[t.SourceID]; !ok {
			out[t.SourceID] = make(TormModels, 0)
		}
		out[t.SourceID] = append(out[t.SourceID], t)
	}
	return out
}

//GetTpl 包含所有define，当使用子模板时有效
func (tModels TormModels) GetTpl() (tpl string) {
	var w bytes.Buffer
	for _, t := range tModels {
		w.WriteString(t.Tpl)
		w.WriteString("\n")
	}
	return w.String()
}

//GetTpl 包含所有define，当使用子模板时有效
func (tModels TormModels) GetTransferLine() (transferLine pathtransfer.TransferLine) {
	var w bytes.Buffer
	for _, t := range tModels {
		w.WriteString(string(t.TransferLine))
		w.WriteString("\n")
	}
	transferLine = pathtransfer.TransferLine(w.String())
	return transferLine
}
