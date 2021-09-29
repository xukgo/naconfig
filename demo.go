package nacosConfig

//type Client struct {
//}
//
//func Start() {
//	// 创建clientConfig
//	clientConfig := constant.ClientConfig{
//		AppName:             "tokenMan",
//		NamespaceId:         "cdfa659c-b7a4-43f6-8519-03233d060bf2", // 如果需要支持多namespace，我们可以场景多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
//		TimeoutMs:           3000,
//		NotLoadCacheAtStart: false,
//		LogDir:              fileUtil.GetAbsUrl("nacos/log"),
//		CacheDir:            fileUtil.GetAbsUrl("nacos/cache"),
//		RotateTime:          "24h",
//		MaxAge:              3,
//		LogLevel:            "warn",
//	}
//
//	// 至少一个ServerConfig
//	serverConfigs := []constant.ServerConfig{
//		{
//			IpAddr:      "172.16.16.212",
//			ContextPath: "/nacos",
//			Port:        80,
//			Scheme:      "http",
//		},
//	}
//
//	// 创建动态配置客户端的另一种方式 (推荐)
//	configClient, err := clients.NewConfigClient(
//		vo.NacosClientParam{
//			ClientConfig:  &clientConfig,
//			ServerConfigs: serverConfigs,
//		},
//	)
//	_ = err
//	//搜索配置
//	configPage, err := configClient.SearchConfig(vo.SearchConfigParam{
//		Search:   "blur",
//		DataId:   "",
//		Group:    "",
//		PageNo:   1,
//		PageSize: 100,
//	})
//	_ = err
//	_ = configPage
//	//fmt.Println(configPage)
//	configClient.ListenConfig(vo.ConfigParam{
//		DataId: "mysql",
//		Group:  "tscloud",
//		OnChange: func(namespace, group, dataId, data string) {
//			fmt.Println("onchange ", data)
//		}})
//
//	time.Sleep(time.Second * 5)
//
//	//for i := 0; i < 10; i++ {
//	//	content, err := configClient.GetConfig(vo.ConfigParam{
//	//		DataId: "mysql",
//	//		Group:  "tscloud"})
//	//	fmt.Println(err)
//	//	fmt.Println(content)
//	//}
//}
