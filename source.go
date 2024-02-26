package apifunc

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/apifunc/provider"
	"github.com/suifengpiao14/sqlexec/sqlexecparser"
)

const (
	PROVIDER_SQL_MEMORY = "SQL_MEMORY"
	PROVIDER_SQL        = "SQL"
	PROVIDER_CURL       = "CURL"
	PROVIDER_BIN        = "BIN"
	PROVIDER_REDIS      = "REDIS"
	PROVIDER_RABBITMQ   = "RABBITMQ"
)

//IdentiferRelation 维护映射关系，比如将模板名称映射资源标识，实现模板名称找到资源
type IdentiferRelation struct {
	TemplateName    string `json:"templateName"`
	SourceIdentifer string `json:"sourceIdentifer"`
}
type IdentiferRelationCollection []IdentiferRelation

//Add 确保templateName 唯一
func (rc *IdentiferRelationCollection) AddTemplateIdentiferRelation(templateIdentifer string, sourceIdentifer string) (err error) {
	for _, r := range *rc {
		if r.TemplateName != "" && r.TemplateName == templateIdentifer {
			err = errors.Errorf("template identifer:%s exists", templateIdentifer)
			return err
		}
	}
	r := IdentiferRelation{
		TemplateName:    templateIdentifer,
		SourceIdentifer: sourceIdentifer,
	}
	*rc = append(*rc, r)
	return nil
}
func (rc *IdentiferRelationCollection) GetSourceIdentiferByTemplateIdentifer(templateIdentifer string) (sourceIdentifer string, err error) {
	for _, r := range *rc {
		if r.TemplateName != "" && r.TemplateName == templateIdentifer {
			return r.SourceIdentifer, nil
		}
	}
	err = errors.Errorf("not found source identifer by template identifer: %s", templateIdentifer)
	return "", err

}

type SourcePool struct {
	sourceMap                   map[string]Source
	IdentiferRelationCollection IdentiferRelationCollection
	lock                        sync.Mutex
}

//NewSourcePool 生成资源池
func NewSourcePool() (p *SourcePool) {
	p = &SourcePool{
		sourceMap:                   make(map[string]Source),
		IdentiferRelationCollection: make(IdentiferRelationCollection, 0),
	}
	return p
}

type Source struct {
	Identifer string    `json:"identifer"`
	Type      string    `json:"type"`
	Config    string    `json:"config"`
	Provider  providerI `json:"-"`
	DDL       string    `json:"ddl"`
}

type providerI interface {
	TypeName() string
}

type MemoryDB struct {
	InOutMap map[string]string
}

func (m *MemoryDB) TypeName() string {
	return "memory_db"
}

func (m *MemoryDB) ExecOrQueryContext(ctx context.Context, sql string) (out string, err error) {
	out, ok := m.InOutMap[sql]
	if !ok {
		err = errors.Errorf("not found by sql:%s", sql)
		return "", err
	}
	return out, nil
}

//MakeSource 创建常规资源,方便外部统一调用
func MakeSource(identifer string, typ string, config string, ddl string) (s Source, err error) {
	s = Source{
		Identifer: identifer,
		Type:      typ,
		Config:    config,
		DDL:       ddl,
	}
	var providerImp providerI
	switch s.Type {
	case PROVIDER_SQL:
		providerImp, err = provider.NewDBProvider(s.Config)
		if err != nil {
			return s, err
		}
		if s.DDL != "" { // 注册关联表结构
			dbname, err := sqlexecparser.GetDBNameFromDSN(s.Config)
			if err != nil {
				return s, err
			}
			err = sqlexecparser.RegisterTableByDDL(dbname, s.DDL)
			if err != nil {
				return s, err
			}
		}
		//todo curl , bin 提供者实现
	}
	s.Provider = providerImp
	return s, nil
}

func (p *SourcePool) RegisterSource(s Source) (err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.sourceMap[s.Identifer] = s
	return nil
}

func (p *SourcePool) AddTemplateIdentiferRelation(templateIdentifer string, sourceIdentifer string) (err error) {
	// 资源必须先注册
	_, ok := p.sourceMap[sourceIdentifer]
	if !ok {
		err = errors.Errorf("register source(%s) befor AddTemplateIdentiferRelation", sourceIdentifer)
		return err
	}
	err = p.IdentiferRelationCollection.AddTemplateIdentiferRelation(templateIdentifer, sourceIdentifer)
	if err != nil {
		return err
	}
	return nil
}

func (p *SourcePool) GetBySourceIdentifer(sourceIdentifer string) (source *Source, err error) {
	sourceO, ok := p.sourceMap[sourceIdentifer]
	if !ok {
		err = errors.Errorf("not found source by source identifier: %s", sourceIdentifer)
		return nil, err
	}
	return &sourceO, nil
}
func (p *SourcePool) GetProviderBySourceIdentifer(sourceIdentifer string) (sourceProvider providerI, err error) {
	source, err := p.GetBySourceIdentifer(sourceIdentifer)

	sourceProvider = source.Provider
	return sourceProvider, nil
}
func (p *SourcePool) GetProviderByTemplateIdentifer(templateIdentifier string) (sourceProvider providerI, err error) {
	sourceIdentifer, err := p.IdentiferRelationCollection.GetSourceIdentiferByTemplateIdentifer(templateIdentifier)
	if err != nil {
		return nil, err
	}
	source, ok := p.sourceMap[sourceIdentifer]
	if !ok {
		err = errors.Errorf("not found source by template identifier: %s", templateIdentifier)
		return nil, err
	}
	sourceProvider = source.Provider
	return sourceProvider, nil
}
