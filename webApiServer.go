package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"strconv"
)

type Home struct {
	TunnelProxy map[string]string `yaml:"tunnelProxy" json:"tunnelProxy"`
	Sum         int               `yaml:"sum" json:"sum"`
	Type        map[string]int    `yaml:"type" json:"type"`
	Anonymity   map[string]int    `yaml:"anonymity" json:"anonymity"`
	Country     map[string]int    `yaml:"country" json:"country"`
	Source      map[string]int    `yaml:"source" json:"source"`
}

func Run() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	//首页
	r.GET("/", index)

	//查询
	r.GET("/get", get)

	//删除
	r.GET("/delete", deleteProxy)

	//验证代理
	r.GET("/verify", verify)

	//抓取代理
	r.GET("/spider", spiderUp)

	log.Printf("webApi启动 - 监听IP端口 -> %s\n", conf.Config.Ip+":"+conf.Config.Port)
	err := r.Run(conf.Config.Ip + ":" + conf.Config.Port)
	if err != nil {
		log.Printf("webApi启动启动失败%v", err)
		return
	}

}
func index(c *gin.Context) {
	home := getProxyPoolStats(true)
	httpsIp = getHttpsIp()
	httpIp = getHttpIp()
	socket5Ip = getSocket5Ip()
	ConnectIp = getConnectIp()
	home.TunnelProxy["HTTP"] = httpIp
	home.TunnelProxy["HTTPS"] = httpsIp
	home.TunnelProxy["SOCKET5"] = socket5Ip
	home.TunnelProxy["CONNECT"] = ConnectIp
	jsonByte, _ := json.Marshal(&home)
	jsonStr := string(jsonByte)
	c.String(200, jsonStr)
}
func get(c *gin.Context) {
	if getProxyCount("all", &[]bool{true}[0]) == 0 {
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"代理池是空的\"}"))
		return
	}
	// 读取查询参数
	ty := c.DefaultQuery("type", "all")
	an := c.DefaultQuery("anonymity", "all")
	re := c.DefaultQuery("country", "all")
	co := c.DefaultQuery("count", "1")
	// 解析 count 参数，默认为1
	count, err := strconv.Atoi(co)
	if err != nil {
		c.String(500, "{\"code\": 500, \"msg\": \"count 参数错误\"}")
		return
	}
	// 调用 getProxyIp 函数，获取符合条件的代理
	proxyList, _ := getProxyIp(ty, an, re, count)
	jsonByte, err := json.Marshal(proxyList)
	if err != nil {
		log.Println("JSON 序列化失败：", err)
		c.String(500, "{\"code\": 500, \"msg\": \"内部错误\"}")
		return
	}
	// 返回代理列表
	c.String(200, string(jsonByte))
}
func deleteProxy(c *gin.Context) {
	if getProxyCount("all", nil) == 0 {
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"代理池是空的\"}"))
		return
	}
	ip := c.Query("ip")
	port := c.Query("port")
	protocol := c.Query("protocol")
	i := delProxy(ip, port, protocol)
	c.String(200, fmt.Sprintf("{\"code\": 200, \"count\": %d}", i))
}
func verify(c *gin.Context) {
	if verifyIS {
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"验证中\"}"))
	} else {
		go VerifyProxy()
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"开始验证代理\"}"))
	}
}

func spiderUp(c *gin.Context) {
	if run {
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"抓取中\"}"))
	} else {
		go spiderRun()
		c.String(200, fmt.Sprintf("{\"code\": 200, \"msg\": \"开始抓取代理IP\"}"))
	}
}
