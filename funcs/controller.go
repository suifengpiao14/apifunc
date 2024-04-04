package funcs

import (
	"context"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/spf13/cast"
	"github.com/suifengpiao14/torm"
)

func Pagination(ctx context.Context, totalTorm torm.Torm, listTorm torm.Torm, input []byte) (out []byte, err error) {
	totalJson, err := totalTorm.Run(ctx, input)
	if err != nil {
		return nil, err
	}
	totalStr, err := totalTorm.TrimOutNamespace(totalJson)
	if err != nil {
		return nil, err
	}
	total := cast.ToInt(totalStr)
	if total == 0 {
		return totalJson, nil
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
