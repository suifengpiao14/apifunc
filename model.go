package apifunc

import (
	"encoding/json"
	"strings"

	"github.com/suifengpiao14/pathtransfer"
)

type DependentJson string

func (dj DependentJson) Dependents() (dependents Dependents, err error) {
	dependents = make(Dependents, 0)
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
	Flows            string                    `json:"flows"`
}

type ApiModels []ApiModel

type SourceModel struct {
	SourceID   string `json:"sourceId"`
	ENV        string `json:"env"`
	SourceType string `json:"sourceType"`
	Config     string `json:"config"`
	DDL        string `json:"ddl"` //SQL 类型，需要使用cudevent 库时需要配置DDL
}

type SourceModels []SourceModel

type TormModel struct {
	TemplateID       string                    `json:"templateId"`
	Title            string                    `json:"title"`
	SourceID         string                    `json:"sourceId"`
	Tpl              string                    `json:"tpl"`
	Type             string                    `json:"type"`
	PathTransferLine pathtransfer.TransferLine `json:"pathTransfers"`
	Flows            string                    `json:"flows"`
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
