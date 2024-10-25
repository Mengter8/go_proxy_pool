package main

import (
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
)

var conf *Config

type Config struct {
	Spider       []Spider       `yaml:"spider" json:"spider"`
	SpiderPlugin []SpiderPlugin `yaml:"spiderPlugin" json:"spiderPlugin"`
	SpiderFile   []SpiderFile   `yaml:"spiderFile" json:"spiderFile"`
	Proxy        Proxy          `yaml:"proxy" json:"proxy"`
	Config       config         `yaml:"config" json:"config"`
}
type config struct {
	Ip               string `yaml:"ip" json:"ip"`
	Port             string `yaml:"port" json:"port"`
	HttpTunnelPort   string `yaml:"httpTunnelPort" json:"httpTunnelPort"`
	SocketTunnelPort string `yaml:"socketTunnelPort" json:"socketTunnelPort"`
	TunnelTime       int    `yaml:"tunnelTime" json:"tunnelTime"`
	ProxyNum         int    `yaml:"proxyNum" json:"proxyNum"`
	VerifyTime       int    `yaml:"verifyTime" json:"verifyTime"`
	VerifyUrl        string `yaml:"verifyUrl" json:"verifyUrl"`
	VerifyUrlWords   string `yaml:"verifyUrlWords" json:"verifyUrlWords"`
	ThreadNum        int    `yaml:"threadNum" json:"threadNum"`
}
type Spider struct {
	Name     string            `yaml:"name" json:"name"`
	Method   string            `yaml:"method" json:"method"`
	Interval int               `yaml:"interval" json:"interval"`
	Body     string            `yaml:"body" json:"body"`
	ProxyIs  bool              `yaml:"proxy" json:"proxy"`
	Headers  map[string]string `yaml:"headers" json:"headers"`
	Urls     []string          `yaml:"urls" json:"urls"`
	Ip       string            `yaml:"ip" json:"ip"`
	Port     string            `yaml:"port" json:"port"`
}
type SpiderPlugin struct {
	Name string `yaml:"name" json:"name"`
	Run  string `yaml:"run" json:"run"`
}
type SpiderFile struct {
	Name string `yaml:"name" json:"name"`
	Path string `yaml:"path" json:"path"`
}

type Proxy struct {
	Host string `yaml:"host" json:"host"`
	Port string `yaml:"port" json:"port"`
}

// 数组去重
func uniquePI(arr []ProxyIp) []ProxyIp {
	seen := make(map[string]struct{})
	var pr []ProxyIp
	for _, v := range arr {
		key := v.IPAddress + v.Port
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			pr = append(pr, v)
		}
	}
	return pr
}

// 读取配置文件
func GetConfigData() {
	//导入配置文件
	yamlFile, err := os.ReadFile("config.yml")
	if err != nil {
		log.Println("配置文件打开错误：" + err.Error())
		err.Error()
		return
	}
	//将配置文件读取到结构体中
	err = yaml.Unmarshal(yamlFile, &conf)
	if err != nil {
		log.Println("配置文件解析错误：" + err.Error())
		err.Error()
		return
	}
}

// 处理Headers配置
func SetHeadersConfig(he map[string]string, header *http.Header) *http.Header {
	for k, v := range he {
		header.Add(k, v)
	}
	return header
}
