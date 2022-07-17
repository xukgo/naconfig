package naconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/xukgo/gsaber/utils/arrayUtil"

	"github.com/nacos-group/nacos-sdk-go/model"

	"github.com/nacos-group/nacos-sdk-go/clients/config_client"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type Repo struct {
	locker       *sync.RWMutex
	config       *ConfRoot
	localDict    map[string]string
	configClient config_client.IConfigClient
}

func (this Repo) FormatConfigDescription() string {
	str := fmt.Sprintf("server:%s; namespaceID:%s", this.config.FormatEndpoints(), this.config.Local.NameSpaceID)
	return str
}

func (this *Repo) InitFromXmlPath(path string, matchHandlers []MatchVarHandler) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return this.InitFromReader(file, matchHandlers)
}

func (this *Repo) InitFromReader(srcReader io.Reader, matchHandlers []MatchVarHandler) error {
	var reader *bufio.Reader
	reader = bufio.NewReader(srcReader)
	buff := make([]byte, 0, 4096)
	if reader == nil {
		return fmt.Errorf("reader is invalid nil")
	}
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			buff = append(buff, line...)
			break
		}
		if err != nil {
			return err
		}
		buff = append(buff, line...)
	}

	conf := new(ConfRoot)
	err := conf.FillWithXml(buff)
	if err != nil {
		return err
	}

	errMsg := conf.CheckValid()
	if len(errMsg) > 0 {
		return fmt.Errorf("配置格式错误:%s", errMsg)
	}

	err = fillHandler(conf, matchHandlers)

	this.locker = new(sync.RWMutex)
	this.localDict = make(map[string]string)
	this.config = conf

	return this.initParam()
}

func fillHandler(conf *ConfRoot, handlers []MatchVarHandler) error {
	for idx := range conf.SubscribeVars {
		subVar := &conf.SubscribeVars[idx]
		handler := findHandlerByName(handlers, subVar.HandlerName)
		if handler == nil {
			return fmt.Errorf("cannot find handler by name:%s", subVar.HandlerName)
		}
		subVar.Handler = handler
	}
	return nil
}

func findHandlerByName(handlers []MatchVarHandler, name string) func(group, dataId, data string) {
	for _, m := range handlers {
		if strings.EqualFold(m.Name, name) {
			return m.Handler
		}
	}
	return nil
}

func (this *Repo) initParam() error {
	if this.config == nil {
		return fmt.Errorf("conf is nil")
	}
	conf := this.config
	procs := runtime.GOMAXPROCS(0)
	if procs > 4 {
		procs = 4
	}

	cacheDir, _ := filepath.Abs(conf.Local.CacheConfig.Dir)
	logDir, _ := filepath.Abs(conf.Local.LogConfig.Dir)
	// 创建clientConfig
	clientConfig := constant.ClientConfig{
		AppName:             conf.Local.AppName,
		NamespaceId:         conf.Local.NameSpaceID, // 如果需要支持多namespace，我们可以场景多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
		TimeoutMs:           uint64(conf.Local.Timeout),
		BeatInterval:        int64(conf.Local.BeatInterval),
		UpdateThreadNum:     procs,
		NotLoadCacheAtStart: conf.Local.CacheConfig.NotLoadAtStart,
		CacheDir:            cacheDir, //fileUtil.GetAbsUrl("nacos/cache"),
		Username:            conf.Local.Authorization.UserName,
		Password:            conf.Local.Authorization.Password,
		LogDir:              logDir, //fileUtil.GetAbsUrl("nacos/log"),
		LogLevel:            conf.Local.LogConfig.Level,
		LogRollingConfig: &constant.ClientLogRollingConfig{
			MaxAge:    conf.Local.LogConfig.MaxAge,
			LocalTime: true,
			Compress:  false,
		},
	}
	if conf.Local.OfflineMode {
		clientConfig.TimeoutMs = 1
		clientConfig.BeatInterval = 1000 * 3600 * 5
	}

	// 至少一个ServerConfig
	serverConfigs := make([]constant.ServerConfig, 0, len(conf.Endpoints))
	for _, item := range this.config.Endpoints {
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr:      item.IP,
			ContextPath: item.Context,
			Port:        uint64(item.Port),
			Scheme:      item.Scheme,
		})
	}

	// 创建动态配置客户端的另一种方式 (推荐)
	configClient, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return err
	}

	this.locker.Lock()
	this.configClient = configClient
	this.locker.Unlock()
	return nil
}

func (this *Repo) Publish(group, id, content string) error {
	if this.configClient == nil {
		return fmt.Errorf("configClient is nil")
	}
	if this.config.Local.OfflineMode {
		return fmt.Errorf("config is offline mode")
	}
	br, err := this.configClient.PublishConfig(vo.ConfigParam{
		DataId:  id,
		Group:   group,
		Content: content,
	})
	_ = br
	return err
}

func (this *Repo) Subscribe(block bool) error {
	if this.config.Local.OfflineMode {
		return this.subscribeOffline()
	}

	if block {
		return this.subscribeOnline()
	}

	go this.subscribeOnline()
	return nil
}

func (this *Repo) subscribeOffline() error {
	for _, svar := range this.config.SubscribeVars {
		if svar.CheckBlur() {
			continue
		}
		content, err := this.configClient.GetConfig(vo.ConfigParam{
			DataId: svar.ID,
			Group:  svar.Group})
		if err != nil {
			return fmt.Errorf("get config error:group[%s] dataID[%s]; %w", svar.Group, svar.ID, err)
		}

		k := this.formatVarKey(svar.Group, svar.ID)
		this.addVar(k, content)
		handler := svar.Handler
		if handler != nil {
			handler(svar.Group, svar.ID, content)
		}
	}
	return nil
}

func (this *Repo) subscribeOnline() error {
	var err error
	dict := make(map[string]SubscribeVar)

	locker := new(sync.Mutex)
	list := make([]string, 0, 32)
	for _, svar := range this.config.SubscribeVars {
		//精确查找
		if !svar.CheckBlur() {
			k := this.formatVarKey(svar.Group, svar.ID)
			_, find := dict[k]
			if find {
				continue
			}
			dict[k] = svar
			h := svar.Handler
			err = this.configClient.ListenConfig(vo.ConfigParam{
				DataId: svar.ID,
				Group:  svar.Group,
				OnChange: func(namespace, group, dataId, data string) {
					k := this.formatVarKey(group, dataId)
					this.addVar(k, data)
					if h != nil {
						h(group, dataId, data)
					}

					locker.Lock()
					list = append(list, k)
					locker.Unlock()
				},
			})
			if err != nil {
				return fmt.Errorf("listen config error:group[%s] dataID[%s]; %w", svar.Group, svar.ID, err)
			}
			continue
		}

		vars := make([]model.ConfigItem, 0, 64)
		pageIndex := 1
		for {
			//模糊查找
			configPage, err := this.configClient.SearchConfig(vo.SearchConfigParam{
				Search:   "blur",
				DataId:   svar.ID,
				Group:    svar.Group,
				PageNo:   pageIndex,
				PageSize: 1000,
			})
			if err != nil {
				return err
			}
			vars = append(vars, configPage.PageItems...)
			if len(configPage.PageItems) < 1000 {
				break
			}
			pageIndex++
		}

		for _, item := range vars {
			k := this.formatVarKey(item.Group, item.DataId)
			_, find := dict[k]
			if find {
				continue
			}
			dict[k] = svar
			h := svar.Handler
			err = this.configClient.ListenConfig(vo.ConfigParam{
				DataId: svar.ID,
				Group:  svar.Group,
				OnChange: func(namespace, group, dataId, data string) {
					k := this.formatVarKey(group, dataId)
					this.addVar(k, data)
					if h != nil {
						h(group, dataId, data)
					}

					locker.Lock()
					list = append(list, k)
					locker.Unlock()
				},
			})
			if err != nil {
				return fmt.Errorf("listen config error:group[%s] dataID[%s]; %w", svar.Group, svar.ID, err)
			}
		}
	}

	for k, v := range dict {
		locker.Lock()
		index := arrayUtil.ContainsString(list, k)
		locker.Unlock()
		if index >= 0 {
			continue
		}
		content, err := this.configClient.GetConfig(vo.ConfigParam{
			DataId: v.ID,
			Group:  v.Group})
		if err != nil {
			return fmt.Errorf("get config error:group[%s] dataID[%s]; %w", v.Group, v.ID, err)
		}

		this.addVar(k, content)
		h := v.Handler
		if h != nil {
			h(v.Group, v.ID, content)
		}
	}
	return nil
}

func (this *Repo) checkVarExist(group, id string) bool {
	this.locker.RLock()
	_, find := this.localDict[this.formatVarKey(group, id)]
	if find {
		this.locker.RUnlock()
		return true
	}
	this.locker.RUnlock()
	return false
}

func (this *Repo) getVar(gourp, id string) string {
	this.locker.RLock()
	v, find := this.localDict[this.formatVarKey(gourp, id)]
	this.locker.RUnlock()
	if !find {
		return ""
	}
	return v
}

func (this *Repo) addVar(k string, content string) {
	this.locker.Lock()
	this.localDict[k] = content
	this.locker.Unlock()
}

func (this *Repo) formatVarKey(group, id string) string {
	k := fmt.Sprintf("%s::%s", group, id)
	return k
}
