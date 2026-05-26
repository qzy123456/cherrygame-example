package data

import (
	"cherry-game/examples/demo_cluster/internal/types"

	cherryMapStructure "github.com/cherry-game/cherry/extend/mapstructure"
	cherryDataConfig "github.com/cherry-game/components/data-config"
)

var (
	AreaConfig       = &areaConfig{}
	AreaGroupConfig  = &areaGroupConfig{}
	AreaServerConfig = &areaServerConfig{}
	SdkConfig        = &sdkConfig{}
	CodeConfig       = &codeConfig{}
	PlayerInitConfig = &playerInitConfig{}
)

func New() *cherryDataConfig.Component {
	dataConfig := cherryDataConfig.New()
	dataConfig.Register(
		AreaConfig,
		AreaGroupConfig,
		AreaServerConfig,
		SdkConfig,
		CodeConfig,
		PlayerInitConfig,
	)
	return dataConfig
}

func DecodeData(input interface{}, output interface{}) error {
	return cherryMapStructure.HookDecode(
		input,
		output,
		"json",
		types.GetDecodeHooks(),
	)
}
