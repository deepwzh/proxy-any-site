package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/deepwzh/proxy-any-site/db"
	"github.com/deepwzh/proxy-any-site/static"
	"github.com/deepwzh/proxy-any-site/util"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type Conf struct {
	Host   string
	Port   int
	Scheam string
}

var (
	conf Conf
)

var (
	dbClient *db.DbClient
)

func CommResp(c *gin.Context, data any) {
	c.JSON(200, Response[any]{
		Code: 0,
		Msg:  "",
		Data: data,
	})
}

func BadResp(c *gin.Context, msg string) {
	c.JSON(400, Response[any]{
		Code: -1,
		Msg:  msg,
	})
}
func init() {
	var host, schema *string
	var port *int
	host = flag.String("host", "", "Host address")
	port = flag.Int("port", 0, "Port number")
	schema = flag.String("schema", "https", "Schema")

	// 解析命令行参数
	flag.Parse()

	conf.Host = *host
	conf.Port = *port
	conf.Scheam = *schema

	// 检查必需的参数是否提供
	if conf.Host == "" || conf.Port == 0 {
		logrus.Panicf("Missing required arguments, %+v", conf)
	}
}

func init() {
	logrus.SetLevel(logrus.TraceLevel)
	var err error
	dbClient, err = db.NewDbClient()
	if err != nil {
		panic(err)
	}
}

func GetShortedHash(url string) (string, error) {
	hash, err := dbClient.GetOriginalUrl(url)
	if err != nil {
		hash = util.ShortenURL(url)
		if err := dbClient.UpdateDomain(url, hash); err != nil {
			return "", err
		}
		return hash, nil
	}

	return hash, nil
}

func getProxyUrl(originalUrl string) (*url.URL, error) {
	hash, err := GetShortedHash(originalUrl)
	if err != nil {
		return nil, err
	}
	return url.ParseRequestURI(fmt.Sprintf("%v://%v.%v", conf.Scheam, hash, conf.Host))
}

func getOriginalUrl(proxyUrl *url.URL) (string, error) {
	// fmt.Printf("shortURL:%v", shortURL)
	shortURL := strings.Split(proxyUrl.Host, ".")[0]
	originalUrl, err := dbClient.GetOriginalUrl(shortURL)
	url := fmt.Sprintf("%v%v", originalUrl, proxyUrl.Path)
	if proxyUrl.RawQuery != "" {
		url = url + "?" + proxyUrl.RawQuery
	}
	return url, err
}

func getAuthHeader(data string) string {
	re := regexp.MustCompile(`realm="([^"]+)"`)
	match := re.FindStringSubmatch(data)
	if len(match) <= 0 {
		return data
	}
	realmValue := match[1]

	oldUrl, err := url.ParseRequestURI(realmValue)
	if err != nil {
		logrus.Error("fail to parse request url, err: %v", err)
		return data
	}

	proxyUrl, err := getProxyUrl(oldUrl.Scheme + "://" + oldUrl.Host)
	if err != nil {
		logrus.Error("fail to get proxyHost, err: %v", err)
		return data
	} else {
		proxyUrl.Path = oldUrl.Path
		replaced := re.ReplaceAllString(data, fmt.Sprintf(`realm="%v"`, proxyUrl.String()))
		return replaced
	}
}

func createProxy(c *gin.Context) {
	domain := c.Query("domain")
	if domain != "" {
		c.Status(400)
	}
	url, err := getProxyUrl(domain)
	if err != nil {
		BadResp(c, err.Error())
		return
	} else {
		CommResp(c, map[string]any{
			"proxy_uri": url.String(),
		})
		return
	}
}

type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

func staticFileRoute(c *gin.Context) {
	file, _ := static.FS.ReadFile("index.html")
	c.Data(
		http.StatusOK,
		"content-type: application/html",
		file,
	)
}

func reverseProxy(c *gin.Context) {
	if c.Request.Host == "www."+conf.Host {
		if c.Request.URL.Path == "/generate" {
			createProxy(c)
			return
		}
		staticFileRoute(c)
		return
	}

	if !strings.HasSuffix(c.Request.Host, "."+conf.Host) {
		BadResp(c, fmt.Sprintf("invaild host, %v", c.Request.Host))
		return
	}

	proxyUrl, _ := url.ParseRequestURI(fmt.Sprintf("%v://%v%v?%v", conf.Scheam, c.Request.Host, c.Request.URL.Path, c.Request.URL.RawQuery))

	url, err := getOriginalUrl(proxyUrl)
	if err != nil {
		logrus.Errorf("fail to get original host, host: %v, err: %v", proxyUrl.Host, err)
		c.Status(http.StatusBadRequest)
		return
	}

	logrus.Infof("request url: %v", url)

	redirectFunc := func(req *http.Request, via []*http.Request) error {
		proxyUrl, err := getProxyUrl(req.URL.Scheme + "://" + req.URL.Host)
		if err != nil {
			logrus.Errorf("fail to get proxyHost, err: %v", err)
			return nil
		} else {
			preReqURL := *req.URL
			req.URL.Host = proxyUrl.Host
			req.URL.Scheme = proxyUrl.Scheme

			logrus.Infof("redirect %v -> %v\n", &preReqURL, req.URL)
		}
		return nil
	}
	_ = redirectFunc

	client := http.Client{
		CheckRedirect: redirectFunc,
	}
	targetReq, err := http.NewRequest(c.Request.Method, url, c.Request.Body)
	if err != nil {
		logrus.Errorf("Error creating target request: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	targetReq.Header = c.Request.Header.Clone()

	targetResp, err := client.Do(targetReq)
	if err != nil {
		logrus.Errorf("Error making target request: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	for key, values := range targetResp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	auth := targetResp.Header.Get("Www-Authenticate")
	preAuth := auth
	if strings.HasPrefix(auth, "Bearer realm") {
		auth = getAuthHeader(auth)
		logrus.Infof("replace Www-Authenticate: %v -> %v", preAuth, auth)
		c.Header("Www-Authenticate", auth)
	}

	c.Status(targetResp.StatusCode)

	_, err = io.Copy(c.Writer, targetResp.Body)
	if err != nil {
		logrus.Errorf("Failed to copy response body: %v", err)
		return
	}

}

func main() {
	logrus.Infof("start reverse proxy sevice...")
	r := gin.Default()

	// r.POST("", createProxy)
	r.Any("/*path", reverseProxy)
	r.Run(fmt.Sprintf("%v:%v", "0.0.0.0", conf.Port)) // 监听并在 0.0.0.0:8080 上启动服务
}
