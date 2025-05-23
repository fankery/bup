package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func init() {
	loggerInit()
	configInit()
}

// 日志初始化
func loggerInit() {
	write := zapcore.AddSync(os.Stdout)
	//日志格式配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000000")
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	core := zapcore.NewCore(encoder, write, zapcore.DebugLevel)
	// logger := zap.New(core, zap.AddCaller())
	logger := zap.New(core)
	Logger = logger.Sugar()
}

func configInit() {
	var dir, _ = os.Executable()
	var absPath, _ = filepath.Abs(filepath.Dir(dir))
	var path = strings.ReplaceAll(absPath, `\`, `/`)
	viper.SetConfigFile(path + "/conf/application.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		Logger.Fatalf("fatal error reading config file: %s", err)
	}
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		Logger.Warnf("config file changed: %s \n", e.Name)
	})
}
