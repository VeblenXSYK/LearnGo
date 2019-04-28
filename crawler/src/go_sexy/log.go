package main

import (
	"bufio"
	"log"
	"os"
)

//日志信息
type logInfo struct {
	logType int
	content string
}

//日志类型
const (
	pageProcessed = iota //已解析网页
	pageFound            //发现的网址
	imgProcessed         //已下载图片
	imgFound             //发现的图片
	appExit              //结束
)

//初始化日志写入器
func InitLogWriter(ctx *context) {
	logFiles := OpenLogFiles(ctx.savePath)
	writers := NewWriters(logFiles)

	for {
		info := <-ctx.logChan
		if info.logType == appExit {
			break
		}
		//根据日志类型写入不同的文件
		writers[info.logType].WriteString(info.content + "\n")
	}

	//关闭文件
	for i, file := range logFiles {
		writers[i].Flush()
		file.Close()
	}
}

//打开日志文件
func OpenLogFiles(savePath string) (files []*os.File) {
	fileNames := []string{"page_processed.log", "page_found.log", "img_processed.log", "img_found.log"}
	files = make([]*os.File, len(fileNames))
	for i, name := range fileNames {
		logFile, err := os.OpenFile(savePath+"/logs/"+name, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Fatalln("An error occurred with file :" + err.Error())
		}
		files[i] = logFile //保存打开文件的File
	}
	return files
}

//创建日志writer
func NewWriters(logFiles []*os.File) []*bufio.Writer {
	writer := make([]*bufio.Writer, len(logFiles))
	for i, file := range logFiles {
		writer[i] = bufio.NewWriter(file)
	}
	return writer
}

//写入日志
func WriteLog(content string, logType int, ctx *context) {
	ctx.logChan <- &logInfo{logType, content}
}
