package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

var httpIp string
var httpsIp string
var ConnectIp string
var socket5Ip string

func httpRunTunnelProxyServer() {
	log.Println("HTTP 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.HttpTunnelPort)

	server := &http.Server{
		Addr: conf.Config.Ip + ":" + conf.Config.HttpTunnelPort,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleHTTPSProxy(w, r)
			} else {
				handleHTTPProxy(w, r)
			}
		}),
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("HTTP监听启动失败: %v", err)
	}
}

func handleHTTPSProxy(w http.ResponseWriter, r *http.Request) {
	ConnectIp = getConnectIp()
	log.Printf("隧道代理 | HTTPS 请求：%s 使用代理: %s", r.URL.String(), ConnectIp)
	destConn, err := net.DialTimeout("tcp", ConnectIp, 20*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		log.Println("代理连接失败:", err)
		return
	}
	defer destConn.Close()

	var reqBuffer bytes.Buffer
	fmt.Fprintf(&reqBuffer, "%s %s %s\r\n", r.Method, r.Host, r.Proto)
	for k, v := range r.Header {
		fmt.Fprintf(&reqBuffer, "%s: %s\r\n", k, v[0])
	}
	reqBuffer.WriteString("\r\n")
	destConn.Write(reqBuffer.Bytes())

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Println("Hijacking connection失败:", err)
		return
	}
	defer clientConn.Close()

	go func() { _, _ = io.Copy(destConn, clientConn) }()
	_, _ = io.Copy(clientConn, destConn)
}

func handleHTTPProxy(w http.ResponseWriter, r *http.Request) {
	httpIp := getHttpIp()
	log.Printf("隧道代理 | HTTP 请求：%s 使用代理: %s", r.URL.String(), httpIp)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse("http://" + httpIp)
		},
	}

	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}
	request, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "请求创建失败"+err.Error(), http.StatusInternalServerError)
		return
	}
	request.Header = r.Header

	res, err := client.Do(request)
	if err != nil {
		http.Error(w, "代理请求失败: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer res.Body.Close()

	for k, values := range res.Header {
		for _, value := range values {
			w.Header().Add(k, value)
		}
	}
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
}

func httpsRunTunnelProxyServer() {
	log.Println("HTTPS 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.HttpsTunnelPort)

	li, err := net.Listen("tcp", conf.Config.Ip+":"+conf.Config.HttpsTunnelPort)
	if err != nil {
		log.Fatalf("HTTPS监听启动失败: %v", err)
		return
	}
	defer li.Close()

	for {
		clientConn, err := li.Accept()
		if err != nil {
			log.Printf("接受客户端连接失败: %v", err)
			continue
		}

		go func(clientConn net.Conn) {
			defer clientConn.Close()

			httpsIp := getHttpsIp()
			log.Printf("隧道代理 | HTTPS 请求使用代理: %s", httpsIp)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			destConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", httpsIp, tlsConfig)
			if err != nil {
				log.Printf("连接到目标代理失败: %v", err)
				return
			}
			defer destConn.Close()

			// 使用 io.Copy 双向复制数据
			done := make(chan struct{})
			go func() {
				io.Copy(destConn, clientConn)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(clientConn, destConn)
				done <- struct{}{}
			}()

			<-done // 等待一个方向完成后关闭连接
			log.Printf("连接已关闭: %s", httpsIp)
		}(clientConn)
	}
}

func socket5RunTunnelProxyServer() {
	log.Println("SOCKET5 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.SocketTunnelPort)
	li, err := net.Listen("tcp", conf.Config.Ip+":"+conf.Config.SocketTunnelPort)
	if err != nil {
		log.Fatalf("SOCKET5监听启动失败: %v", err)
		return
	}
	defer li.Close()

	for {
		clientConn, err := li.Accept()
		if err != nil {
			log.Printf("接受客户端连接失败: %v", err)
			continue
		}

		go func(clientConn net.Conn) {
			defer clientConn.Close()

			socket5Ip = getSocket5Ip()
			if socket5Ip == "" {
				log.Println("未能获取有效的 SOCKET5 代理 IP")
				return
			}
			log.Printf("隧道代理 | SOCKET5 请求 使用代理: %s", socket5Ip)
			destConn, err := net.DialTimeout("tcp", socket5Ip, 30*time.Second)
			if err != nil {
				log.Printf("连接到代理失败: %v", err)
				return
			}
			defer destConn.Close()

			// 设置读取和写入的超时时间
			clientConn.SetDeadline(time.Now().Add(30 * time.Second))
			destConn.SetDeadline(time.Now().Add(30 * time.Second))

			// 使用 io.Copy 双向传输数据
			done := make(chan struct{})
			go func() {
				io.Copy(destConn, clientConn)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(clientConn, destConn)
				done <- struct{}{}
			}()

			<-done // 等待一方完成即可
			log.Println("SOCKET5 代理连接关闭")
		}(clientConn)
	}
}
