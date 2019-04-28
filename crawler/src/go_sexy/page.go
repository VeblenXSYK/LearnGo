package main

import (
	"bytes"
	"go_sexy/conf"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	titleExp       = regexp.MustCompile(`<title>([^<>]+)</title>`) //regexp.MustCompile(`<img\s+src="([^"'<>]*)"/?>`)
	invalidCharExp = regexp.MustCompile(`[\\/*?:><|]`)
)

//一个需要解析的页面
type page struct {
	url   string  //页面地址
	body  *[]byte //html数据
	retry int     //重试次数
	parse bool    //true时将不检查url是否符合config中配置的正则表达式
}

//抓取html页面
func (p *page) pollPage(ctx *context, client *http.Client) {

	ctx.lockPage.RLock()

	//判断该页面是否已解析
	if ctx.pageMap[p.url] == done {
		ctx.lockPage.RUnlock()
		return
	} else {
		ctx.lockPage.RUnlock()
	}

	//函数返回时重新解析没有成功解析的页面
	defer p.retryPage(ctx)

	req, err := http.NewRequest("GET", p.url, nil)
	for k, v := range ctx.config.Header {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Print("pollPage[1]:" + err.Error())
		return
	}
	defer resp.Body.Close()

	reader := resp.Body.(io.Reader)
	if ctx.config.Charset == "gbk" {
		reader = transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder()) //字符转码
	}

	//获取该html页面
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Print("pollPage[2]:" + err.Error())
		return
	}

	ctx.lockPage.Lock()
	ctx.pageMap[p.url] = done
	ctx.lockPage.Unlock()

	//开始解析html页面
	p.body = &body
	ctx.parseChan <- p
}

//失败后重新把页面放入channel
func (p *page) retryPage(ctx *context) {

	ctx.lockPage.RLock()
	if ctx.pageMap[p.url] == done {
		ctx.lockPage.RUnlock()
		return
	} else {
		ctx.lockPage.RUnlock()
	}

	//注意：golang不支持++i和--i
	if p.retry++; p.retry < maxRetry {
		go func() {
			ctx.pageChan <- p
		}()
	} else {
		ctx.lockPage.Lock()
		ctx.pageMap[p.url] = fail
		ctx.lockPage.Unlock()
	}
}

//解析html页面
func (p *page) parsePage(ctx *context) {

	//符合正则表达式的图片页面
	if matchUrl(p.url, ctx.config.ImgPageRegex) {
		log.Println("match ImagePage URL : " + p.url)
		for _, exp := range ctx.config.ImageExp {
			p.findImage(ctx, exp)
		}
	}

	//符合正则表达式的页面
	if matchUrl(p.url, ctx.config.PageRegex) || p.parse {
		log.Println("match Page URL : " + p.url)
		for _, exp := range ctx.config.HrefExp {
			p.findURL(ctx, exp)
		}
	}

	WriteLog(p.url, pageProcessed, ctx)
}

//在html页面中查找图片地址
func (p *page) findImage(ctx *context, exp *conf.MatchExp) {

	pageUrl, err := url.Parse(p.url)
	if err != nil {
		log.Fatal("findImage[1]:" + err.Error())
		return
	}

	//将网页str转化为dom对象
	body := *(p.body)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	folder := p.createImageFolder(ctx, exp)
	doc.Find(exp.Query).Each(func(i int, s *goquery.Selection) {

		imgUrlStr, e := s.Attr(exp.Attr)
		if !e || imgUrlStr == "" {
			return
		}

		absUrl := toAbsUrl(pageUrl, imgUrlStr) //转换成绝对地址
		absUrl.Fragment = ""                   //删除锚点
		imgUrlStr = absUrl.String()

		//检查该imgUrl是否已放入map，这里需要同步
		ctx.lockImg.RLock()
		_, exist := ctx.imgMap[imgUrlStr]
		ctx.lockImg.RUnlock()

		if !exist {
			ctx.lockImg.Lock()
			ctx.imgMap[imgUrlStr] = ready
			ctx.lockImg.Unlock()

			log.Println("imgUrlStr:", imgUrlStr)
			fileName := path.Base(p.url) + "_" + strconv.Itoa(i) + path.Ext(imgUrlStr)
			ctx.imgChan <- &image{imgUrlStr, fileName, 0, folder}
		}
	})
}

//在html页面中查找链接
func (p *page) findURL(ctx *context, exp *conf.MatchExp) {

	pageUrl, err := url.Parse(p.url)
	if err != nil {
		log.Print("findURL[1]:" + err.Error())
		return
	}

	body := *(p.body)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(exp.Query).Each(func(i int, s *goquery.Selection) {
		linkUrlStr, e := s.Attr(exp.Attr)
		if !e || linkUrlStr == "" {
			return
		}
		linkUrl := toAbsUrl(pageUrl, linkUrlStr)
		if linkUrl == nil || linkUrl.Host != ctx.rootURL.Host {
			return
		}
		linkUrl.Fragment = ""
		linkUrlStr = linkUrl.String()

		ctx.lockPage.RLock()
		_, exist := ctx.pageMap[linkUrlStr]
		ctx.lockPage.RUnlock()

		if !exist {
			ctx.lockPage.Lock()
			ctx.pageMap[linkUrlStr] = ready
			ctx.lockPage.Unlock()

			go func() { //这里必须异步，不然会和pollPage互相等待造成死锁
				ctx.pageChan <- &page{url: linkUrlStr}
			}()
		}
	})
}

//创建图片文件夹
func (p *page) createImageFolder(ctx *context, reg *conf.MatchExp) string {

	var folder string
	body := *(p.body)

	fd, ok := reg.Folder.(regexp.Regexp)
	if ok {
		//folder为正则表达式
		loc := fd.FindIndex(body)
		if loc == nil {
			return ctx.savePath
		}
		folder = string(body[loc[0]:loc[1]])
	} else {
		fdstr, ok := reg.Folder.(string)
		if !ok {
			return ctx.savePath
		}
		switch fdstr {
		//folder为图片所在页面url的name
		case "url":
			folder = path.Base(p.url)
			if folder == "" {
				folder = "root"
			}
		//folder为所在页面的title
		case "title":
			loc := titleExp.FindSubmatchIndex(body)
			if loc == nil {
				return ctx.savePath
			}
			folder = string(body[loc[2]:loc[3]])
		//不建文件夹，所有图片都放在一起
		case "none":
			return ctx.savePath
		}
	}

	folder = invalidCharExp.ReplaceAllString(folder, "")
	folder = ctx.savePath + "/" + folder + "/"
	err := os.Mkdir(folder, 0777)
	if err != nil {
		return ctx.savePath
	}
	return folder
}

//判断正则表达式是否能匹配制定的url
func matchUrl(url string, reglist []*regexp.Regexp) bool {
	if reglist == nil || len(reglist) == 0 {
		return true
	}
	for _, reg := range reglist {
		if reg.MatchString(url) {
			return true
		}
	}
	return false
}

//转换成绝对地址
func toAbsUrl(pageURL *url.URL, href string) *url.URL {
	//.  ..  /  ? http https
	var buf bytes.Buffer
	if h := strings.ToLower(href); strings.Index(h, "http://") == 0 || strings.Index(h, "https://") == 0 {
		buf.WriteString(href)
	} else {
		buf.WriteString(pageURL.Scheme)
		buf.WriteString("://")
		buf.WriteString(pageURL.Host)

		switch href[0] {
		case '?':
			if len(pageURL.Path) == 0 {
				buf.WriteByte('/')
			} else {
				buf.WriteString(pageURL.Path)
			}
			buf.WriteString(href)
		case '/':
			buf.WriteString(href)
		default:
			p := "/" + path.Dir(pageURL.Path) + "/" + href
			buf.WriteString(path.Clean(p))
		}
	}

	h, err := url.Parse(buf.String())
	if err != nil {
		log.Print("toAbsUrl[1]:" + err.Error())
		return nil
	}
	return h
}
