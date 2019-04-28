package main

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"os"
)

//一张需要下载的图片
type image struct {
	imageURL string //图片地址
	fileName string //保存到本地的文件名
	retry    int    //重试次数
	folder   string //存放的文件夹
}

//下载图片
func (imgInfo *image) downloadImage(ctx *context, client *http.Client) {
	imgUrl := imgInfo.imageURL

	ctx.lockImg.RLock()
	if ctx.imgMap[imgUrl] == done {
		ctx.lockImg.RUnlock()
		return
	} else {
		ctx.lockImg.RUnlock()
	}
	defer imgInfo.imageRetry(ctx) //失败时重试

	req, err := http.NewRequest("GET", imgUrl, nil)
	for k, v := range ctx.config.Header {
		req.Header.Add(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Print("downloadImage[1]:" + err.Error())
		return
	}
	defer resp.Body.Close()

	//fmt.Println("download:" + imgUrl)
	saveFile := imgInfo.folder + imgInfo.fileName //path.Base(imgUrl)

	img, err := os.Create(saveFile)
	if err != nil {
		log.Print("downloadImage[2]:" + err.Error())
		return
	}
	defer img.Close()

	imgWriter := bufio.NewWriterSize(img, bufferSize)

	_, err = io.Copy(imgWriter, resp.Body)
	if err != nil {
		log.Print("downloadImage[3]:" + err.Error())
		return
	}
	imgWriter.Flush()

	ctx.lockImg.Lock()
	ctx.imgMap[imgUrl] = done
	ctx.lockImg.Unlock()
	ctx.imgCount <- 1
	WriteLog(imgUrl, imgProcessed, ctx)
}

//失败重试
func (imgInfo *image) imageRetry(ctx *context) {
	ctx.lockImg.RLock()
	if ctx.imgMap[imgInfo.imageURL] == done {
		ctx.lockImg.RUnlock()
		return
	} else {
		ctx.lockImg.RUnlock()
	}
	if imgInfo.retry++; imgInfo.retry < maxRetry {
		go func() { //异步发送，避免阻塞
			ctx.imgChan <- imgInfo
		}()
	} else {
		ctx.lockImg.Lock()
		ctx.imgMap[imgInfo.imageURL] = fail
		ctx.lockImg.Unlock()
	}
}
