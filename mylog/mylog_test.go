package mylog

import (
	"testing"
	"github.com/pion/logging"
)

func TestDefaultLogger(t *testing.T) {
	Logger.WithOutput("./11.log")
	Logger.SetLevel(logging.LogLevelWarn)
	Logger.Warn("miaobinwei")
	Logger.Warnf("miaobinwei %d % d %s",1,2,"aaaa")
	Logger.Warnf("miaobinwei %d % d %s",1,2,"aaaa")
}
