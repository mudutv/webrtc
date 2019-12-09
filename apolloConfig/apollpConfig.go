package apolloConfig

import (
	"fmt"
	"git.mudu.tv/myun/myun-apollo/sdk"
	"github.com/pion/webrtc/mylog"
	"os"
	"github.com/pion/logging"
)

var (
	apolloAppId     = Env("APOLLO_APP_ID", "myun-apollo-mrtc-go")
	apolloCluster   = Env("APOLLO_CLUSTER", "dev")
	apolloIp        = Env("APOLLO_IP", "service-apollo-config-server.myun:8080")
	apolloNamespace = Env("APOLLO_NAMESPACE", "application")
	apolloCacheDir  = Env("APOLLO_CACHE_DIR", "apollo_cache")
)

func Env(key string, defaultValue string) string {
	v, exist := os.LookupEnv(key)
	if !exist {
		return defaultValue
	}
	return v
}

type AppConfig struct {
	GrpcPort     string `apollo_key:"GRPC_PORT" apollo_callback:"OnDBChange" apollo_default:"60000"`
	BaseUrl      string    `apollo_key:"BASE_URL" apollo_callback:"OnDBChange" apollo_default:"https://198dev.myun.tv/mrtc-gateway"`
	LogPath      string `apollo_key:"LOG_PATH" apollo_callback:"OnDBChange" apollo_default:"./main.log"`
	LogLv        int `apollo_key:"LOG_LV" apollo_callback:"OnDBChange" apollo_default:"3"`
}

var G_Config *AppConfig

func StartApollo() bool{
	fmt.Println("start config")
	err := apollo.StartWithConfAndStrategy(&apollo.Conf{
		AppID:          apolloAppId,
		Cluster:        apolloCluster,
		NameSpaceNames: []string{"application"},
		IP:             apolloIp,
		CacheDir:       apolloCacheDir,
	}, &AppConfig{})
	if err != nil {
		fmt.Println("start config fail 1")
		G_Config = &AppConfig{GrpcPort:"60000", BaseUrl:"https://198dev.myun.tv/mrtc-gateway",LogPath:"./main.log", LogLv:3}
		mylog.Loginit(G_Config.LogPath,G_Config.LogLv)
		mylog.Logger.Errorf("init StartWithConfAndStrategy fail err[%s]",err.Error())
		return false
	}

	config, err := apollo.GetCurrentConfig()
	if err != nil {
		fmt.Println("start config fail 2")
		G_Config = &AppConfig{GrpcPort:"60000", BaseUrl:"https://198dev.myun.tv/mrtc-gateway",LogPath:"./main.log", LogLv:3}
		mylog.Loginit(G_Config.LogPath,G_Config.LogLv)
		mylog.Logger.Errorf("init GetCurrentConfig fail err[%s]",err.Error())
		return false
	}else{
		G_Config = config.(*AppConfig)
		fmt.Println("start config success  [%v]",*G_Config)
		mylog.Loginit(G_Config.LogPath,G_Config.LogLv)
		mylog.Logger.Infof("init GetCurrentConfig success  config[%v]",*G_Config)
	}
	return true
}

func (c *AppConfig) OnDBChange(config AppConfig) {
	*G_Config = *c
	mylog.Logger.Warnf("new:[%v] old:[%v]", *G_Config, config)
	if c.LogLv != config.LogLv{
		mylog.Logger.SetLevel(logging.LogLevel(c.LogLv))
	}
}

