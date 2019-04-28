// sexy project main.go
package main

import (
	"fmt"
	"go_sexy/conf"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type context struct {
	lockPage  sync.RWMutex
	pageMap   map[string]int //记录已处理的页面，key是地址，value是处理状态
	lockImg   sync.RWMutex
	imgMap    map[string]int //记录已处理的图片
	pageChan  chan *page     //待抓取的网页channel
	imgChan   chan *image    //待下载的图片channel
	parseChan chan *page     //待解析的网页channel
	imgCount  chan int       //统计已下载完成的图片
	logChan   chan *logInfo  //日志
	savePath  string         //图片存放的路径
	rootURL   *url.URL       //起始地址，从这个页面开始爬
	config    *conf.Config   //配置信息
}

const (
	bufferSize     = 64 * 1024       //写图片文件的缓冲区大小
	numPoller      = 10              //抓取网页的并发数
	numDownloader  = 10              //下载图片的并发数
	maxRetry       = 2               //抓取网页或下载图片失败时重试的次数
	statusInterval = 5 * time.Second //进行状态监控的间隔
	chanBufferSize = 10              //待解析的
)

const (
	//图片或页面处理状态
	ready = iota //待处理
	done         //已处理
	fail         //失败
)

func main() {

	configFile := "config.json"
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	}

	cf := &conf.Config{}
	if err := cf.Load(configFile); err != nil {
		panic("some error occurred when loading the config file:" + err.Error())
	}

	client, err := InitHttpClient(cf)
	if err != nil {
		panic("some error occurred while initializing the http client:" + err.Error())
	}

	fmt.Println("start download...")

	ctx := InitContext(cf)

	StartCrawler(ctx, client)
	StateMonitor(ctx)
}

func InitContext(cf *conf.Config) (ctx *context) {
	savePath := "./" + cf.Root.Host + "/"
	os.MkdirAll(savePath+"logs", 0777)
	log.Println(savePath)

	return &context{
		pageMap:   make(map[string]int),
		imgMap:    make(map[string]int),
		pageChan:  make(chan *page, chanBufferSize*10),
		imgChan:   make(chan *image, chanBufferSize*20),
		parseChan: make(chan *page, chanBufferSize),
		imgCount:  make(chan int),
		logChan:   make(chan *logInfo, 20),
		savePath:  savePath,
		rootURL:   cf.Root,
		config:    cf,
	}
}

//开启网页爬虫
func StartCrawler(ctx *context, client *http.Client) {

	go InitLogWriter(ctx)

	//抓取html页面（因为有多个goroutine，所以需要对pageMap的操作做同步）
	for i := 0; i < numPoller; i++ {
		go func() {
			for {
				p := <-ctx.pageChan
				p.pollPage(ctx, client)
			}
		}()
	}

	//解析html页面
	go func() {
		for {
			p := <-ctx.parseChan
			p.parsePage(ctx)
		}
	}()

	//下载图片
	for i := 0; i < numDownloader; i++ {
		go func() {
			for {
				img := <-ctx.imgChan
				img.downloadImage(ctx, client)
			}
		}()
	}

	//放入起始页面，开始工作了
	ctx.pageChan <- &page{url: ctx.config.Root.String(), parse: true}
}

//初始化http客户端
func InitHttpClient(cf *conf.Config) (*http.Client, error) {
	if strings.TrimSpace(cf.Proxy.Server) != "" { //使用代理
		var auth *proxy.Auth
		if strings.TrimSpace(cf.Proxy.UserName) != "" {
			auth = &proxy.Auth{User: cf.Proxy.UserName, Password: cf.Proxy.Password}
		}
		dialer, err := proxy.SOCKS5("tcp", cf.Proxy.Server,
			auth,
			&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			},
		)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			Proxy:               nil,
			Dial:                dialer.Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		log.Println("Use proxy server:" + cf.Proxy.Server)
		return &http.Client{Transport: transport}, nil
	}
	return &http.Client{}, nil //不使用代理
}

//状态监控
func StateMonitor(ctx *context) {
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(statusInterval) //定时向ticker.C发送信息
	count := 0
	isDone := true
	logFormat := "========================================================\n"
	logFormat += "queue:page(%v)\timage(%v)\tparse(%v)\nimage:found(%v)\tdone(%v)\n"
	logFormat += "========================================================\n"
	for {
		select {
		case <-ticker.C:
			fmt.Printf(logFormat, len(ctx.pageChan), len(ctx.imgChan), len(ctx.parseChan), len(ctx.imgMap), count)
			if len(ctx.pageChan) == 0 && len(ctx.imgChan) == 0 && len(ctx.parseChan) == 0 {
				//当所有channel都为空，并且所有图片都已下载则退出程序
				isDone = true
				for _, val := range ctx.imgMap {
					if val == ready {
						isDone = false
						break
					}
				}
				if isDone {
					WriteLog("", appExit, ctx)
					time.Sleep(3 * time.Second)
					fmt.Println("is done!")
					os.Exit(0)
				}
			}
		case c := <-ctx.imgCount:
			count += c //统计下载成功的图片数量
		}
	}
}
