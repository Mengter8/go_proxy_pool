package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var wg3 sync.WaitGroup
var ch1 = make(chan int, 50)

func init() {
	// 设置全局日志格式和输出
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
}

func main() {
	fmt.Println("           ___                     ___            _ " +
		"\n ___  ___ | . " + "\\ _ _  ___ __   _ _ | . \\ ___  ___ | |" +
		"\n/ . |/ . \\|  _/| '_>/ . \\\\ \\/| | ||  _// . \\/ . \\| |" +
		"\n\\_. |\\___/|_|  |_|  \\___//\\_\\`_. ||_|  \\___/\\___/|_|" +
		"\n<___'                        <___'                  ")
	initSqlite()
	InitData()
	var a, b, c, d bool
	a = VerifyProxy2("127.0.0.1:20171", "HTTP")
	b = VerifyProxy2("127.0.0.1:20171", "HTTPS")
	b = VerifyProxy2("127.0.0.1:20171", "HTTPS")
	c = VerifyProxy2("127.0.0.1:20171", "CONNECT")
	d = VerifyProxy2("127.0.0.1:20170", "SOCKET5")
	log.Printf("http: %v, https: %v, connect: %v, socket5: %v", a, b, c, d)
	//开启隧道代理
	go httpRunTunnelProxyServer()
	go httpsRunTunnelProxyServer()
	go socket5RunTunnelProxyServer()
	//启动webAPi
	Run()
}

// 初始化
func InitData() {
	//获取配置文件
	GetConfigData()
	//设置线程数量
	ch1 = make(chan int, conf.Config.ThreadNum)
	ch2 = make(chan int, conf.Config.ThreadNum)
	//是否需要抓代理
	if len(ProxyPool) < conf.Config.ProxyNum {
		//抓取代理
		spiderRun()
	}
	//定时判断是否需要获取代理iP
	go func() {
		// 每 60 秒钟时执行一次
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			if len(ProxyPool) < conf.Config.ProxyNum {
				if !run && !verifyIS {
					log.Printf("代理数量不足 %d\n", conf.Config.ProxyNum)
					//抓取代理
					spiderRun()
				}
			}
		}
	}()

	// 验证代理存活情况
	go func() {
		verifyTime := time.Duration(conf.Config.VerifyTime)
		ticker := time.NewTicker(verifyTime * time.Second)
		for range ticker.C {
			if !verifyIS && !run {
				VerifyProxy()
			}
		}
	}()
}
