package naconfig

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ConfRoot struct {
	XMLName       xml.Name
	EnvDefine     EnvironmentDefine `xml:"EnvDefine"`
	Endpoints     []ServerEndpoint  `xml:"Servers>Endpoint"` //
	Local         *LocalConf        `xml:"Local"`            //etcd连接超时时间,单秒秒
	SubscribeVars []SubscribeVar    `xml:"Subscribe>Var"`    //
}

func (this ConfRoot) FormatEndpoints() string {
	sb := strings.Builder{}
	for idx, v := range this.Endpoints {
		sb.WriteString(v.FormatUrl())
		if idx != len(this.Endpoints)-1 {
			sb.WriteByte(',')
		}
	}
	return sb.String()
}

func (this ConfRoot) CheckValid() string {
	if this.Local == nil {
		return "local config is nil"
	}
	errMsg := this.Local.CheckValid()
	if len(errMsg) > 0 {
		return errMsg
	}
	if len(this.Endpoints) == 0 {
		return "Endpoint config is empty"
	}
	for _, endpoint := range this.Endpoints {
		errMsg = endpoint.CheckValid()
		if len(errMsg) > 0 {
			return errMsg
		}
	}
	if len(this.SubscribeVars) == 0 {
		return "Subscribe Vars is empty"
	}
	for _, svar := range this.SubscribeVars {
		errMsg = svar.CheckValid()
		if len(errMsg) > 0 {
			return errMsg
		}
	}
	return ""
}

func (this *ConfRoot) FillWithXml(xmlContents []byte) error {
	err := xml.Unmarshal(xmlContents, this)
	if err != nil {
		return err
	}

	if len(this.EnvDefine.NacosUrls) == 0 {
		return nil
	}
	urlstrs := os.Getenv(this.EnvDefine.NacosUrls)
	if len(urlstrs) == 0 {
		return nil
	}
	sarr := strings.Split(urlstrs, ",")

	this.Endpoints = make([]ServerEndpoint, 0, len(sarr))
	for _, str := range sarr {
		str = strings.TrimSpace(str)
		u, err := url.Parse(str)
		if err != nil {
			return fmt.Errorf("parse env[%s] url[%s] return error:%w", this.EnvDefine.NacosUrls, str, err)
		}
		var port int64 = 0
		portStr := u.Port()
		if len(portStr) == 0 {
			if u.Scheme == "http" {
				port = 80
			} else {
				port = 443
			}
		} else {
			port, err = strconv.ParseInt(portStr, 10, 64)
			if err != nil {
				return fmt.Errorf("parse env[%s] url[%s] return error:%w", this.EnvDefine.NacosUrls, str, err)
			}
		}
		this.Endpoints = append(this.Endpoints, ServerEndpoint{
			IP:      u.Host,
			Port:    int(port),
			Context: u.Path,
			Scheme:  u.Scheme,
		})
	}
	if len(this.EnvDefine.NacosNamespace) > 0 && this.Local != nil {
		namespace := os.Getenv(this.EnvDefine.NacosNamespace)
		this.Local.NameSpaceID = namespace
	}
	return nil
}

type EnvironmentDefine struct {
	NacosUrls      string `xml:"NacosUrls"`
	NacosNamespace string `xml:"NacosNamespace"`
}

type LocalConf struct {
	AppName       string               `xml:"AppName"`
	NameSpaceID   string               `xml:"NameSpaceID"`
	Timeout       int                  `xml:"Timeout"`      //请求超时ms
	BeatInterval  int                  `xml:"BeatInterval"` //和服务器的心跳间隔ms
	CacheConfig   *CacheConf           `xml:"Cache"`        //
	Authorization *ClientAuthorization `xml:"Auth"`         //
	LogConfig     *LocalLog            `xml:"Log"`
	OfflineMode   bool                 `xml:"OfflineMode"`
}

type CacheConf struct {
	Dir            string `xml:"dir,attr"`
	NotLoadAtStart bool   `xml:"notLoadAtStart,attr"`
}
type ClientAuthorization struct {
	UserName string `xml:"username,attr"`
	Password string `xml:"password,attr"`
}
type LocalLog struct {
	Dir      string `xml:"dir,attr"`
	Rotation string `xml:"rotation,attr" `
	MaxAge   int    `xml:"maxAge,attr" `
	Level    string `xml:"level,attr" `
}

type ServerEndpoint struct {
	IP      string `xml:"ip,attr" `
	Port    int    `xml:"port,attr" `
	Context string `xml:"context,attr"`
	Scheme  string `xml:"scheme,attr" `
}

func (this ServerEndpoint) FormatUrl() string {
	return fmt.Sprintf("%s://%s:%d%s", this.Scheme, this.IP, this.Port, this.Context)
}

type SubscribeVar struct {
	Group       string                           `xml:"group,attr"`
	ID          string                           `xml:"id,attr"`
	HandlerName string                           `xml:"handler,attr"`
	Handler     func(group, dataId, data string) `xml:"-"`
}

type MatchVarHandler struct {
	Name    string
	Handler func(group, dataId, data string)
}

func InitMatchVarHandler(name string, h func(group, dataId, data string)) MatchVarHandler {
	return MatchVarHandler{
		Name:    name,
		Handler: h,
	}
}

func (this LocalConf) CheckValid() string {
	if len(this.AppName) == 0 {
		return "local config AppName is empty"
	}
	if this.Timeout < 1 {
		return "local config Timeout invalid"
	}
	if this.BeatInterval < 1 {
		return "local config BeatInterval invalid"
	}
	if this.CacheConfig == nil {
		return "local cache config is nil"
	}
	if this.Authorization == nil {
		return "local authorization config is nil"
	}
	if this.LogConfig == nil {
		return "local log config is nil"
	}
	errMsg := this.CacheConfig.CheckValid()
	if len(errMsg) > 0 {
		return errMsg
	}
	errMsg = this.LogConfig.CheckValid()
	if len(errMsg) > 0 {
		return errMsg
	}
	return ""
}

func (this CacheConf) CheckValid() string {
	if len(this.Dir) == 0 {
		return "cache config dir is empty"
	}
	return ""
}
func (this LocalLog) CheckValid() string {
	if len(this.Dir) == 0 {
		return "log config dir is empty"
	}
	if len(this.Rotation) == 0 {
		return "log rotation dir is empty"
	}
	if this.MaxAge < 1 {
		return "log maxAge invalid"
	}
	if len(this.Level) == 0 {
		return "log level is empty"
	}
	return ""
}

func (this ServerEndpoint) CheckValid() string {
	if len(this.IP) == 0 {
		return "server IP is empty"
	}
	if this.Port < 1 || this.Port > 65535 {
		return "server Port invalid"
	}
	if len(this.Scheme) == 0 {
		return "server Scheme is empty"
	}
	return ""
}

func (this SubscribeVar) CheckValid() string {
	if len(this.Group) == 0 {
		return "subscribe var group is empty"
	}
	if len(this.ID) == 0 {
		return "subscribe var id is empty"
	}
	if len(this.HandlerName) == 0 {
		return "subscribe var handler is empty"
	}
	return ""
}

func (this SubscribeVar) CheckBlur() bool {
	if strings.Index(this.Group, "*") >= 0 {
		return true
	}
	if strings.Index(this.ID, "*") >= 0 {
		return true
	}
	return false
}
