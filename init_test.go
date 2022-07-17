package naconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	vhs := make([]MatchVarHandler, 0, 1)
	vhs = append(vhs, InitMatchVarHandler("configHandler", updateAppConfig))

	storage := new(Repo)
	xmlPath, _ := filepath.Abs("nacos.xml")
	err := storage.InitFromXmlPath(xmlPath, vhs)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func updateAppConfig(group, dataId, data string) {
	fmt.Printf("get update data from: %s.%s.%s\n", group, dataId, data)
}
