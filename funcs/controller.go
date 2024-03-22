package funcs

import (
	"context"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/spf13/cast"
	"github.com/suifengpiao14/torm"
	"github.com/tidwall/gjson"
)

func Pagination(ctx context.Context, totalTorm torm.Torm, listTorm torm.Torm, input []byte) (out []byte, err error) {
	totalJson, err := totalTorm.Run(ctx, input)
	if err != nil {
		return nil, err
	}

	//"{\"Dictionary\":{\"pagination\":{\"total\":29}}}"
	totalPath := "Dictionary.pagination.total"
	total := cast.ToInt(gjson.GetBytes(totalJson, totalPath).String())
	if total == 0 {
		return nil, nil
	}
	paginationJson, err := listTorm.Run(ctx, input)
	if err != nil {
		return nil, err
	}
	out, err = jsonpatch.MergePatch([]byte(paginationJson), []byte(totalJson))
	if err != nil {
		return nil, err
	}
	return out, nil
}
