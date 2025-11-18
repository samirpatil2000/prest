package main

import (
	"fmt"

	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/tenantconfig"
	"github.com/structy/log"
)

func main() {
	config.Load()
	if err := tenantconfig.LoadDefault(); err != nil {
		log.Errorln("failed to load tenant configuration: %v", err)
	} else {
		log.Debugln("tenant configuration loaded")
	}
	fmt.Println("tenantConfigMap => ", tenantconfig.TenantConfigMap["loopx"].DBURL)
	cmd.Execute()
}
