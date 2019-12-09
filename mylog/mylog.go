package mylog

import (
	"fmt"
	"github.com/pion/logging"
	"os"
	"os/exec"
	"time"
)

type LogHandle struct {
	 Log *logging.DefaultLeveledLogger
	 writeFile *os.File
}

var filename string = "./main.log"
var Logger *LogHandle

func Loginit(filepath string, level int) {
	//var f *os.File
	//var err1 error
	//filename := "./sfu.log"
	//
	//if checkFileIsExist(filename) { //如果文件存在
	//	f, err1 = os.OpenFile(filename, os.O_APPEND |os.O_RDWR, 0666) //打开文件
	//	if nil != err1{
	//		fmt.Println(err1)
	//	} else{
	//		fmt.Println(filename, "文件存在")
	//	}
	//
	//} else {
	//	f, err1 = os.Create(filename) //创建文件
	//	if nil != err1{
	//		fmt.Println(err1)
	//	} else{
	//		fmt.Println(filename, "文件不存在")
	//	}
	//}

	Logger = new(LogHandle)
	Logger.Log = logging.
		NewDefaultLeveledLoggerForScope("main", logging.LogLevel(level), os.Stdout)

	Logger.WithOutput(filepath)
	Logger.Infof("begin init logging filepath[%s]\n",filepath)
	go func() {
		Logger.Error("begin init reset \n")
		filename = filepath
		for range time.Tick(500 * time.Millisecond) {
			Logger.Reset()
		}
	}()
}


func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func (log *LogHandle)WithOutput(filename string){
	var f *os.File
	var err1 error

	if checkFileIsExist(filename) { //如果文件存在
		f, err1 = os.OpenFile(filename, os.O_APPEND |os.O_RDWR, 0666) //打开文件
		if nil != err1{
			fmt.Println(err1)
			panic("init log file fail 1")
		} else{
			fmt.Println(filename, "is exit")
		}

	} else {
		f, err1 = os.Create(filename) //创建文件
		if nil != err1{
			fmt.Println(err1)
			panic("init log file fail 2")
		} else{
			fmt.Println(filename, "not exit")
		}
	}

	Logger.Log.WithOutput(f)
	Logger.writeFile = f
	Logger.Log.SetFileName(filename)
	//Logger.SetLevel(logging.LogLevelInfo)
}

func (log *LogHandle)Mv(src,dst string) bool{
	cmd := exec.Command("mv", "-f", src,dst)

	err := cmd.Run()
	if (nil != err){
		log.Warnf("file Mv src[%s] to dst [%s] fail", src,dst)
		return false
	}
	return true
}

func (log *LogHandle)Reset(){
	var f *os.File
	var err1 error

	//fileinfo, err := os.Stat(`C:\Users\Administrator\Desktop\UninstallTool.zip`)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println(fileinfo.Name())    //获取文件名
	//fmt.Println(fileinfo.IsDir())   //判断是否是目录，返回bool类型
	//fmt.Println(fileinfo.ModTime()) //获取文件修改时间
	//fmt.Println(fileinfo.Mode())
	//fmt.Println(fileinfo.Size()) //获取文件大小
	//fmt.Println(fileinfo.Sys())
	filepath := log.Log.GetFileName()
	if checkFileIsExist(filepath) { //如果文件存在
		fileinfo, err := os.Stat(filepath)
		if err != nil{
			log.Warnf("file reset  os.Stat filepath [%s] err[%v] fail",filepath,err)
			return
		}

		byteslen := fileinfo.Size()
		if byteslen > 1024*1024*50{
			dstfile := filepath + "-" +time.Now().Format("2006-01-02 15:04:05")
			log.Warnf("file reset mv src[%s] to dstfile[%s] begin",filepath,dstfile)
			ok := log.Mv(filepath, dstfile)
			if true == ok{
				f, err1 = os.Create(filename) //创建文件
				if nil != err1{
					fmt.Println(err1)
					fmt.Println("file reset mv new file fail")
				} else{
					Logger.Log.WithOutput(f)
					if (nil != Logger.writeFile){
						Logger.writeFile.Close()
					}
					Logger.writeFile = f
					log.Warnf("file reset mv src[%s] to dstfile[%s] success",filepath,dstfile)
				}
			}
		}
		return
	} else {
		f, err1 = os.Create(filename) //创建文件
		if nil != err1{
			fmt.Println(err1)
			fmt.Println("file reset new file fail")
		} else{
			Logger.Log.WithOutput(f)
			if (nil != Logger.writeFile){
				Logger.writeFile.Close()
			}
			Logger.writeFile = f
			log.Warnf("file reset [%s] success",filepath)
		}
	}


	//Logger.Log.SetFileName(filename)
	//Logger.SetLevel(logging.LogLevelInfo)
}

func (log *LogHandle)SetLevel(newLevel logging.LogLevel){
	Logger.Log.SetLevel(newLevel)
}

func (log *LogHandle)Warn(msg string){
	Logger.Log.Warn(msg)
}

func (log *LogHandle)Warnf(format string, args ...interface{}){
	Logger.Log.Warnf(format, args...)
}

func (log *LogHandle)Debug(msg string){
	Logger.Log.Debug(msg)
}

func (log *LogHandle)Debugf(format string, args ...interface{}){
	Logger.Log.Debugf(format, args...)
}

func (log *LogHandle)Error(msg string){
	Logger.Log.Error(msg)
}

func (log *LogHandle)Errorf(format string, args ...interface{}){
	Logger.Log.Errorf(format, args...)
}

func (log *LogHandle)Info(msg string){
	Logger.Log.Info(msg)
}

func (log *LogHandle)Infof(format string, args ...interface{}){
	Logger.Log.Infof(format, args...)
}

func (log *LogHandle)Trace(msg string){
	Logger.Log.Trace(msg)
}

func (log *LogHandle)Tracef(format string, args ...interface{}){
	Logger.Log.Tracef(format, args...)
}



