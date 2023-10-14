package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

var (
	Host *string
	Port *int
)

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
func reverseProxy(c *gin.Context) {
	// c.String(200, "%s", c.Param("path"))
	targetURL := c.Param("path")
	scheme := c.Param("scheme")

	if scheme != "http" && scheme != "https" {
		referer, err := url.Parse(c.GetHeader("Referer"))
		if err == nil {
			if referer.Scheme == "https" {
				targetURL = referer.Path[6:] + "/" + scheme + targetURL
				scheme = "https"
			} else if referer.Scheme == "http" {
				targetURL = referer.Path[5:] + "/" + scheme + targetURL
				scheme = "http"
			}

		}
	}

	path := fmt.Sprintf("%v:/%v", scheme, targetURL)

	redirectFunc := func(req *http.Request, via []*http.Request) error {

		req.URL.Path = "/" + req.URL.Scheme + "/" + req.URL.Host
		req.URL.Scheme = "http"
		req.URL.Host = fmt.Sprintf("%v:%v", *Host, *Port)
		return nil
	}

	client := http.Client{
		CheckRedirect: redirectFunc,
	}
	targetReq, err := http.NewRequest(c.Request.Method, path, c.Request.Body)
	if err != nil {
		log.Printf("Error creating target request: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	targetReq.Header = c.Request.Header.Clone()

	// // 发起目标请求
	targetResp, err := client.Do(targetReq)
	if err != nil {
		log.Printf("Error making target request: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// // 复制目标响应的 header 到响应中
	for key, values := range targetResp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	c.Status(targetResp.StatusCode)

	_, err = io.Copy(c.Writer, targetResp.Body)
	if err != nil {
		log.Println("Failed to copy response body:", err)
		return
	}

}

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.Any("/:scheme/*path", reverseProxy)
	r.Run(fmt.Sprintf("%v:%v", "0.0.0.0", *Port)) // 监听并在 0.0.0.0:8080 上启动服务
}
