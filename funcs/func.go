package funcs

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 动态脚本中经常使用的函数封装

func GetsetJsonByte(input []byte, gjsonPath string, changeFn func(oldValue string) (newValue string, err error)) (output []byte, err error) {
	outStr, err := GetsetJson(string(input), gjsonPath, changeFn)
	if err != nil {
		return nil, err
	}
	output = []byte(outStr)
	return output, nil
}

//GetsetJson 指定gjson path确定路径，支持子集多次被序列化情况，修改值,常用来修改翻页参数
func GetsetJson(input string, gjsonPath string, changeFn func(oldValue string) (newValue string, err error)) (output string, err error) {
	if gjsonPath == "" { // 最后一级
		output, err = changeFn(input)
		if err != nil {
			return "", err
		}
		return output, nil
	}
	dotIndex := strings.Index(gjsonPath, ".")
	prePath := gjsonPath
	lastPath := ""
	if dotIndex > -1 {
		prePath, lastPath = gjsonPath[:dotIndex], gjsonPath[dotIndex+1:]
	}
	result := gjson.Get(input, prePath)
	sub, err := GetsetJson(result.String(), lastPath, changeFn)
	if err != nil {
		return "", err
	}
	if result.IsObject() || result.IsArray() {
		output, err = sjson.SetRaw(input, prePath, sub)
	} else {
		output, err = sjson.Set(input, prePath, sub)
	}
	return output, err

}
