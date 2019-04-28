# go_sexy

Go语言实现妹子图爬虫

纯粹是一个练手的项目，里面用到很多Go的特性，例如goroutine、channel、自定义类型、错误处理等等

放上来给大家参考一下，我也是初学golang，有哪些地方写得不好请指正

## 更新说明
- 2015年10月20日 增加了配置文件的功能，把要抓取的网站地址和相关的正则表达式放在json配置文件里
- 2017年11月28日 合并了 @hanshijiex 提交的代码，修复一下问题：1、多协程竞态读写map导致panic 2、迁移到妹子图的新网址www.mmjpg.com 3、模拟header，骗过防抓取导致抓到错误图片
- 2017年11月29日 1、支持SOCKS5代理服务器；2、http客户端只创建一次

```javascript
{
	"root":"xxxxxx.com",
	"proxy":{
		"server":"127.0.0.1:1080", /*SOCKS5代理服务器，如果设置成空字符串则不使用代理 127.0.0.1:1080*/
		"username":"",
		"password":""
	},
	"header":{/*http请求头*/
		"Host":"xxxxxx.com",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.94 Safari/537.36",
		"Referer": "http://xxxxxx.com/zaqizaba/2407.html"
	},
	"charset":"gbk", /*可选值utf-8或gbk*/
	"regex":{
		"page":[], /*正则表达式，只有符合的页面才会被抓取并解析，空白表示所有页面都抓取*/
		"imgInPage":["\S+\d+\.html"], /*存放正则，指定图片存在于哪些页面*/
		"href":[ /*匹配页面上的链接*/
			{
				"query":"a", /*存放链接的dom选择器*/
				"attr":"href"
			}
		],
		"image":[ /*匹配页面上的图片地址*/
			{
				"query":"article.article-content img", /*匹配图片的dom选择器*/
				"attr":"src",
				"folder":"none" /*存放图片的文件夹，可选值url,title,none,正则表达式,文件夹名称*/
			}
		]
	}
}
```

**配置文件使用json格式：**
- root:字符串，要抓取的站点地址
- header:HTTP请求头
- charset:指定页面的编码，可选值utf-8或gbk
- proxy.server:代理服务器地址和端口，例如：127.0.0.1:1080，只支持SOCKS5代理服务器，空字符串表示不使用代理
- proxy.username:代理服务器用户名，如果不需要登录则设置空字符串
- proxy.password:代理服务器密码，如果不需要登录则设置空字符串
- regex.image:数组，用于匹配页面上的图片地址
- regex.image.query:字符串，匹配图片的dom选择器
- regex.image.attr:字符串，指定存储图片地址的属性名称
- regex.image.folder:字符串，可输入url，title，none或正则表达式，其中正则表达式用于匹配页面上的内容
    - url：使用图片所在页面的url的name(源码为path.Base(url))做文件夹名称
    - title：使用页面的title
    - none：不建文件夹，所有图片都放在一起
    - 正则表达式：可以匹配页面上的内容来生成文件夹名称
- regex.page:数组，存放正则表达式，只有符合正则表达式的页面才会被抓取并解析，留空表示所有页面都抓取并解析
- regex.imgInPage:数组，存放正则表达式，用于指定图片存在于哪些页面里
- regex.href:数组，用于匹配页面上的超链接
 - regex.href.query:字符串，存放链接的dom选择器
 - regex.href.attr:字符串，指定存储链接地址的属性名称

## 编译说明
- golang.org/x/net包 [下载地址](https://github.com/golang/net/tree/release-branch.go1.9)
- golang.org/x/text包 [下载地址](https://github.com/golang/text)

## 实现原理
![实现原理](http://git.oschina.net/xpan-lu/go_sexy/raw/master/theory.png)