package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var wg3 sync.WaitGroup
var ch1 = make(chan int, 50)
var verifyIS = false
var PublicIp = "0.0.0.0"

func Verify(pi *ProxyIp, wg *sync.WaitGroup, ch chan int, first bool) {
	defer func() {
		wg.Done()
		<-ch
	}()
	// 拼接IP和端口
	proxyAddress := pi.IPAddress + ":" + pi.Port
	// 通用协议验证逻辑
	startTime := time.Now()
	switch {
	case (pi.Protocol == "HTTPS" || first) && VerifyProxy2(proxyAddress, "HTTPS"):
		pi.Protocol = "HTTPS"
		pi.IsWorking = true
	case (pi.Protocol == "CONNECT" || first) && VerifyProxy2(proxyAddress, "CONNECT"):
		pi.Protocol = "CONNECT"
		pi.IsWorking = true
	case (pi.Protocol == "HTTP" || first) && VerifyProxy2(proxyAddress, "HTTP"):
		pi.Protocol = "HTTP"
		pi.IsWorking = true
	case (pi.Protocol == "SOCKET5" || first) && VerifyProxy2(proxyAddress, "SOCKET5"):
		pi.Protocol = "SOCKET5"
		pi.IsWorking = true
	default:
		pi.IsWorking = false
	}
	pi.ResponseTime = time.Since(startTime).Milliseconds()
	if pi.IsWorking && first {
		// 获取匿名级别、ISP等信息并保存
		pi.Anonymity = Anonymity(pi)
		if pi.Anonymity == "" {
			pi.IsWorking = false
		}
		pi.Isp, pi.Country, pi.Province, pi.City = getIpAddressInfo(pi.IPAddress)
		updateProxyRecord(pi)
	} else if pi.Protocol != "" {
		// 仅更新验证时间和评分及状态
		updateProxyRecord(pi)
	}
}

func VerifyProxy2(proxyAddress string, protocol string) bool {
	// 根据代理协议构造代理URL
	var verifyPrefix string
	var tr *http.Transport

	switch protocol {
	case "SOCKET5":
		verifyPrefix = "https://"
		dialer, err := proxy.SOCKS5("tcp", proxyAddress, nil, proxy.Direct)
		if err != nil {
			log.Printf("SOCKET5 代理创建失败: %v", err)
			return false
		}
		tr = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr) // 使用 SOCKS5 的 Dial 方法
			}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	case "HTTP":
		verifyPrefix = "http://"
		proxyUrl, proxyErr := url.Parse("http://" + proxyAddress)
		if proxyErr != nil {
			log.Printf("代理URL解析失败: %v", proxyErr)
			return false
		}
		tr = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	case "HTTPS":
		verifyPrefix = "https://"
		proxyUrl, proxyErr := url.Parse("https://" + proxyAddress)
		if proxyErr != nil {
			log.Printf("代理URL解析失败: %v", proxyErr)
			return false
		}
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyUrl),
		}
	case "CONNECT":
		verifyPrefix = "https://"

		proxyUrl, proxyErr := url.Parse("http://" + proxyAddress)
		if proxyErr != nil {
			log.Printf("代理URL解析失败: %v", proxyErr)
			return false
		}
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyUrl),
		}

	default:
		log.Printf("不支持的协议: %s", protocol)
		return false
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	// 检查 VerifyUrl 和 VerifyUrlWords 是否正确配置
	if conf.Config.VerifyUrl == "" || conf.Config.VerifyUrlWords == "" {
		log.Printf("配置中的 VerifyUrl 或 VerifyUrlWords 不正确")
		return false
	}

	// 创建并发送请求
	request, err := http.NewRequest("GET", verifyPrefix+conf.Config.VerifyUrl, nil)
	if err != nil {
		log.Printf("请求失败: %v", err)
		return false
	}

	response, err := client.Do(request)
	if err != nil {
		//log.Printf("代理请求失败: %v", err)
		return false
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("关闭 response.Body 时出错: %v", err)
		}
	}()
	dataBytes, _ := io.ReadAll(response.Body)
	result := string(dataBytes)
	return strings.Contains(result, conf.Config.VerifyUrlWords)
}
func Anonymity(pr *ProxyIp) string {
	host := "http://httpbin.org/get"
	var proxyAddress string

	// 根据代理协议构造代理URL
	if pr.Protocol == "SOCKET5" {
		proxyAddress = "socks5://" + pr.IPAddress + ":" + pr.Port
	} else if pr.Protocol == "HTTP" || pr.Protocol == "CONNECT" {
		proxyAddress = "http://" + pr.IPAddress + ":" + pr.Port
	} else if pr.Protocol == "HTTPS" {
		proxyAddress = "https://" + pr.IPAddress + ":" + pr.Port
	}

	// 解析代理URL
	proxyUrl, proxyErr := url.Parse(proxyAddress)
	if proxyErr != nil {
		log.Printf("解析代理URL失败: %v", proxyErr)
		pr.IsWorking = false
		return ""
	}

	// 设置代理及请求
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           http.ProxyURL(proxyUrl), // 设置代理
	}
	client := http.Client{Timeout: 15 * time.Second, Transport: &tr}
	request, err := http.NewRequest("GET", host, nil)
	//处理返回结果
	response, err := client.Do(request)
	if err != nil {
		//log.Printf("代理请求失败: %s %v", proxyAddress, err)
		pr.IsWorking = false
		return ""
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("关闭 response.Body 时出错: %v", err)
		}
	}()

	// 读取响应体
	dataBytes, _ := io.ReadAll(response.Body)
	result := string(dataBytes)

	// 判断代理匿名性
	arr := regexp.MustCompile(`"origin": "(.*)",`).FindStringSubmatch(result)
	if len(arr) == 0 {
		//log.Printf("响应不符合预期: %s %s", proxyAddress, result)
		pr.IsWorking = false
		return ""
	}
	origin := arr[1]
	if strings.Contains(origin, PublicIp) {
		pr.IsWorking = true
		return "透明" // 透明代理，暴露了真实IP
	}
	if strings.Contains(origin, pr.IPAddress) {
		pr.IsWorking = true
		return "普匿" // 普通匿名代理
	}
	// 如果请求成功但没有匹配到特定条件，视为高匿名
	pr.IsWorking = true
	return "高匿"
}
func getIpAddressInfo(IpAddres string) (string, string, string, string) {
	var Isp, Country, Province, City = "", "", "", ""

	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Timeout: 15 * time.Second, Transport: &tr}
	//处理返回结果
	response, err := client.Get("https://searchplugin.csdn.net/api/v1/ip/get?ip=" + IpAddres)
	if err != nil {
		response, err = client.Get("https://searchplugin.csdn.net/api/v1/ip/get?ip=" + IpAddres)
		if err != nil {
			return Isp, Country, Province, City
		}
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("关闭 response.Body 时出错: %v", err)
		}
	}()
	dataBytes, _ := io.ReadAll(response.Body)
	result := string(dataBytes)
	address := regexp.MustCompile("\"address\":\"(.+?)\",").FindAllStringSubmatch(result, -1)
	if len(address) != 0 {
		addresss := removeDuplication_map(strings.Split(address[0][1], " "))
		le := len(addresss)
		Isp = strings.Split(addresss[le-1], "/")[0]
		for i := range addresss {
			if i == le-1 {
				break
			}
			switch i {
			case 0:
				Country = addresss[0]
			case 1:
				Province = addresss[1]
			case 2:
				City = addresss[2]
			}
		}
	}
	return Isp, Country, Province, City
}

// 获取公网IP
func getPublicIp() {
	// 发起HTTP请求
	response, err := http.Get("http://myexternalip.com/raw")
	if err != nil {
		fmt.Println("请求出错:", err)
		return
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("关闭 response.Body 时出错: %v", err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("读取响应内容出错:", err)
		return
	}
	PublicIp = string(body)
	// 打印获取到的外部IP
	log.Printf("外部IP: %s", body)
}

func VerifyProxy() {
	if run {
		log.Println("代理抓取中, 无法进行代理验证")
		return
	}
	verifyIS = true
	log.Printf("开始验证代理存活情况")
	getPublicIp()
	ProxyPool := getProxyPool(nil)
	for i := range ProxyPool {
		wg3.Add(1)
		ch1 <- 1
		go Verify(&ProxyPool[i], &wg3, ch1, false)
	}
	time.Sleep(15 * time.Second)

	wg3.Wait()
	log.Printf("代理验证结束, 当前可用IP数: %d\n", len(ProxyPool))
	cleanInvalidProxies()
	verifyIS = false
}

func removeDuplication_map(arr []string) []string {
	set := make(map[string]struct{}, len(arr))
	j := 0
	for _, v := range arr {
		_, ok := set[v]
		if ok {
			continue
		}
		set[v] = struct{}{}
		arr[j] = v
		j++
	}

	return arr[:j]
}
