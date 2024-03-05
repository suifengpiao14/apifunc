package capiprovider_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/suifengpiao14/apifunc/capiprovider"
)

func TestLoadFromXmlDB(t *testing.T) {
	env := "dev"
	apiDir := `./example/xmldb/api`
	sourceDir := `./example/xmldb/source`
	tormDir := `./example/xmldb/template`
	apiModels, sourceModels, tormModels, err := capiprovider.LoadXmlDB(env, apiDir, sourceDir, tormDir)
	require.NoError(t, err)
	fmt.Println(apiModels)
	fmt.Println(tormModels)
	err = sourceModels.FillDDL()
	require.NoError(t, err)
	fmt.Println(sourceModels)
}
