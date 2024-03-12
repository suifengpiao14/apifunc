package apifunc

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/pathtransfer"
)

type SelectFuncTransfer map[string]pathtransfer.Transfers

// GetRequiredTransfers 获取一个必选的转换器(map[xx] 对应只有一条可选转换器时，该转换器必选)并将SelectFuncTransfer中涉及该函数可以提供的转换关系删除
func (sfT SelectFuncTransfer) GetRequiredTransfers() (requiredTransfers pathtransfer.Transfers) {
	requiredTransfers = make(pathtransfer.Transfers, 0)
	for _, transfers := range sfT {
		if len(transfers) == 0 {
			requiredTransfers.AddReplace(transfers[0])
		}
	}
	return requiredTransfers
}

// RemoveTransfers 移除指定的转换器(配合GetRequiredTransfers 使用)
func (sfT *SelectFuncTransfer) RemoveTransfers(requiredTransfers ...pathtransfer.Transfer) {
	subSFT := make(SelectFuncTransfer)
	for _, dstTransfer := range requiredTransfers {
		for key, transfers := range *sfT {
			hasSame := false
			for _, transfer := range transfers {
				if strings.EqualFold(transfer.String(), dstTransfer.String()) {
					hasSame = true
					break
				}
			}
			if !hasSame {
				subSFT[key] = transfers
			}

		}
	}
	*sfT = subSFT
}

// GetTransferFuncByOutputArgs 根据输入的目标转换关系，找出转换函数
func GetTransferFuncByOutputArgs(outputTransfer pathtransfer.Transfers, vocabularies pathtransfer.Transfers) (funcNames []string, err error) {
	funcNames = make([]string, 0)
	selectFuncT := make(SelectFuncTransfer)
	for _, outT := range outputTransfer {
		exit := false
		for _, vT := range vocabularies {
			if strings.EqualFold(vT.Dst.Path, outT.Dst.Path) {
				if _, ok := selectFuncT[vT.Dst.Path]; !ok {
					selectFuncT[vT.Dst.Path] = make(pathtransfer.Transfers, 0)
				}
				selectFuncT[vT.Dst.Path] = append(selectFuncT[vT.Dst.Path], vT)
				exit = true
				//break //这里收集所有可以转换的词汇，后续择优选择，在调用第三方接口时有优势
			}
		}
		if !exit {
			err = errors.Errorf("dictionary vocabulary not defined :%s", outT.Dst.Path)
			return nil, err
		}
	}

	return funcNames, nil
}
