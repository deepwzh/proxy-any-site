package main

import (
	"encoding/base32"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	Host *string
	Port *int
)

func init() {
	logrus.SetLevel(logrus.TraceLevel)
}

func init() {
	Host = flag.String("host", "", "Host address")
	Port = flag.Int("port", 0, "Port number")

	// 解析命令行参数
	flag.Parse()

	// 检查必需的参数是否提供
	if *Host == "" || *Port == 0 {
		panic("Missing required arguments")

	}

}

func getSourceHost(h string) (string, error) {
	t := strings.Split(h, ".")
	s := t[0]
	s = strings.ToUpper(s)
	padding := 8 - (len(s) % 8)
	for i := 0; i < padding%8; i++ {
		s += "="
	}
	decoded, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("decoding error: %v", err)
	}
	return string(decoded), nil
}

func encodeURL(raw string) string {
	s := base32.StdEncoding.EncodeToString([]byte(raw))
	s = strings.TrimRight(s, "=")
	return s
}

func getAuthHeader(data string) string {
	re := regexp.MustCompile(`realm="([^"]+)"`)
	match := re.FindStringSubmatch(data)
	if len(match) <= 0 {
		return data
	}
	realmValue := match[1]
	fmt.Println(realmValue)

	oldUrl, err := url.ParseRequestURI(realmValue)
	if err != nil {
		return data
	}

	newURL := oldUrl
	newURL.Scheme = "https"
	newURL.Host = fmt.Sprintf("%v.%v", encodeURL(oldUrl.Scheme+"://"+oldUrl.Host), *Host)
	// path := url.ParseRequestURI(rawURL)
	replaced := re.ReplaceAllString(data, fmt.Sprintf(`realm="%v"`, newURL.String()))
	return replaced
}

func reverseProxy(c *gin.Context) {
	// c.String(200, "%s", c.Param("path"))
	targetURL := c.Param("path")
	curHost, err := getSourceHost(c.Request.Host)
	if err != nil {
		logrus.Errorf("fail to get source host, host: %v, err: %v", c.Request.Host, err)
		c.Status(http.StatusBadRequest)
		return
	}
	// scheme := "https"

	path := fmt.Sprintf("%v%v", curHost, targetURL)
	if c.Request.URL.RawQuery != "" {
		path = path + "?" + c.Request.URL.RawQuery
	}

	logrus.Infof("request url: %v", path)

	redirectFunc := func(req *http.Request, via []*http.Request) error {
		// req.URL.Path = req.URL.Host
		preReqURL := req.URL
		req.URL.Host = fmt.Sprintf("%v.%v", encodeURL(req.URL.Scheme+"://"+req.URL.Host), *Host)
		req.URL.Scheme = "https"

		logrus.Infof("redirect %v -> %v\n", preReqURL, req.URL)
		return nil
	}

	client := http.Client{
		CheckRedirect: redirectFunc,
	}
	targetReq, err := http.NewRequest(c.Request.Method, path, c.Request.Body)
	if err != nil {
		logrus.Errorf("Error creating target request: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	targetReq.Header = c.Request.Header.Clone()

	// // 发起目标请求
	targetResp, err := client.Do(targetReq)
	if err != nil {
		logrus.Errorf("Error making target request: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// // 复制目标响应的 header 到响应中

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

	// str := `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`
	// replaced := getAuthHeader(str)
	// fmt.Printf("%v", replaced)
	logrus.Infof("start reverse proxy sevice...")
	r := gin.Default()

	r.Any("/*path", reverseProxy)
	r.Run(fmt.Sprintf("%v:%v", "0.0.0.0", *Port)) // 监听并在 0.0.0.0:8080 上启动服务
}
