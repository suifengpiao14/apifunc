package apifunc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/suifengpiao14/lineschema"
)

type Vocabulary struct {
	Fullname   string `json:"fullname"`
	Dictionary string `json:"dictionary"`
}

//Explain 解析元素，分离路径和类型
func (v Vocabulary) Explain() (fullnamePath string, fullnameType string, dictionaryPath string, dictionaryType string) {
	fullnamePath, fullnameType = explainPthType(v.Fullname)
	dictionaryPath, dictionaryType = explainPthType(v.Dictionary)
	return fullnamePath, fullnameType, dictionaryPath, dictionaryType
}

func explainPthType(pathType string) (path string, typ string) {
	typ = "string"
	path = pathType
	name := pathType
	lastIndexDot := strings.LastIndex(name, ".")
	if lastIndexDot > -1 {
		name = name[lastIndexDot+1:]
	}
	indexAt := strings.Index(name, "@")
	if indexAt > -1 {
		ext := name[indexAt:]
		typ = ext[1:]
		path = strings.TrimSuffix(pathType, ext)
	}
	return path, typ
}

const (
	Vocabulary_Fullname_Typ_input  = ".input."
	Vocabulary_Fullname_Typ_output = ".output."
)

type Vocabularies []Vocabulary

func (a Vocabularies) Len() int           { return len(a) }
func (a Vocabularies) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Vocabularies) Less(i, j int) bool { return a[i].Fullname < a[j].Fullname }

func (vs *Vocabularies) AddReplace(vocs ...Vocabulary) {
	m := make(map[string]Vocabulary)
	for _, v := range *vs {
		m[v.Fullname] = v
	}
	for _, v := range vocs {
		m[v.Fullname] = v
	}
	tmp := make(Vocabularies, 0)
	for _, v := range m {
		tmp = append(tmp, v)
	}
	sort.Sort(tmp)
	*vs = tmp
}

func (vs *Vocabularies) GetInOut(namespace string) (in Vocabularies, out Vocabularies) {
	inputPath := fmt.Sprintf("%s%s", namespace, Vocabulary_Fullname_Typ_input)
	inVocabularies := vs.GetByParent(inputPath)
	outputPath := fmt.Sprintf("%s%s", namespace, Vocabulary_Fullname_Typ_output)
	outVocabularies := vs.GetByParent(outputPath)
	return inVocabularies, outVocabularies
}

// GetByParent 获取子集
func (vs *Vocabularies) GetByParent(asParent string) (subVocs Vocabularies) {
	subVocs = make(Vocabularies, 0)
	self := strings.TrimRight(asParent, ".")
	asParent = fmt.Sprintf("%s.", self)
	for _, v := range *vs {
		if strings.EqualFold(v.Fullname, self) {
			subVocs = append(subVocs, v)
			continue
		}
		if strings.HasPrefix(v.Fullname, asParent) {
			subVocs = append(subVocs, v)
		}
	}
	return subVocs
}

// FilterEmpty 为了方便查看，在不同的对象直接会增加一些空格内容，采用此函数过滤掉
func (vs *Vocabularies) FilterEmpty() {
	tmp := make(Vocabularies, 0)
	for _, v := range *vs {
		fullname := strings.Trim(strings.TrimSpace(v.Fullname), "-")
		if fullname == "" {
			continue
		}
		tmp = append(tmp, v)
	}
	*vs = tmp
}

// String 字符串化
func (vs *Vocabularies) String() (s string) {
	b, err := json.Marshal(vs)
	if err != nil {
		panic(err)
	}
	s = string(b)
	return s
}

//Transfers 将fullname作为源，dictionary 作为目标，生成转换关系（适用于输出，输入类型对transfers翻转即可）
func (vs Vocabularies) Transfers() (transfers lineschema.Transfers) {
	transfers = make(lineschema.Transfers, 0)
	for _, v := range vs {
		fullnamePath, fullnameType, dictionaryPath, dictionaryType := v.Explain()
		transfer := lineschema.Transfer{
			Src: lineschema.TransferUnit{
				Path: fullnamePath,
				Type: fullnameType,
			},
			Dst: lineschema.TransferUnit{
				Path: dictionaryPath,
				Type: dictionaryType,
			},
		}
		transfers = append(transfers, transfer)
	}
	return transfers
}
