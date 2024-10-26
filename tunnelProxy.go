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
var socket5Ip string

func httpRunTunnelProxyServer() {
	log.Println("HTTP 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.HttpTunnelPort)

	server := &http.Server{
		Addr:      conf.Config.Ip + ":" + conf.Config.HttpTunnelPort,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.Method == http.MethodConnect {
				httpsIp = getConnectIp()
				log.Printf("隧道代理 | HTTPS 请求：%s 使用代理: %s", r.URL.String(), httpsIp)
				destConn, err := net.DialTimeout("tcp", httpsIp, 20*time.Second)
				if err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}
				destConn.SetReadDeadline(time.Now().Add(20 * time.Second))
				var req []byte
				req = MergeArray([]byte(fmt.Sprintf("%s %s %s%s", r.Method, r.Host, r.Proto, []byte{13, 10})), []byte(fmt.Sprintf("Host: %s%s", r.Host, []byte{13, 10})))
				for k, v := range r.Header {
					req = MergeArray(req, []byte(fmt.Sprintf(
						"%s: %s%s", k, v[0], []byte{13, 10})))
				}
				req = MergeArray(req, []byte{13, 10})
				io.ReadAll(r.Body)
				all, err := io.ReadAll(r.Body)
				if err == nil {
					req = MergeArray(req, all)
				}
				destConn.Write(req)
				w.WriteHeader(http.StatusOK)
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "not supported", http.StatusInternalServerError)
					return
				}
				clientConn, _, err := hijacker.Hijack()
				if err != nil {
					return
				}
				clientConn.SetReadDeadline(time.Now().Add(20 * time.Second))
				destConn.Read(make([]byte, 1024)) //先读取一次
				go io.Copy(destConn, clientConn)
				go io.Copy(clientConn, destConn)

			} else {
				httpIp = getHttpIp()
				log.Printf("隧道代理 | HTTP 请求：%s 使用代理: %s", r.URL.String(), httpIp)
				tr := &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
				//配置代理
				proxyUrl, parseErr := url.Parse("http://" + httpIp)
				if parseErr != nil {
					return
				}
				tr.Proxy = http.ProxyURL(proxyUrl)
				client := &http.Client{Timeout: 20 * time.Second, Transport: tr}
				request, err := http.NewRequest(r.Method, "", r.Body)
				//增加header选项
				request.URL = r.URL
				request.Header = r.Header
				//处理返回结果
				res, err := client.Do(request)
				if err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}
				defer res.Body.Close()

				for k, vv := range res.Header {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				var bodyBytes []byte
				bodyBytes, _ = io.ReadAll(res.Body)
				res.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				w.WriteHeader(res.StatusCode)
				io.Copy(w, res.Body)
				res.Body.Close()
			}
		}),
	}
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
func httpsRunTunnelProxyServer() {
	log.Println("HTTPS 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.HttpsTunnelPort)
	li, err := net.Listen("tcp", conf.Config.Ip+":"+conf.Config.HttpsTunnelPort)
	if err != nil {
		log.Println(err)
	}
	for {
		clientConn, err := li.Accept()
		if err != nil {
			log.Panic(err)
		}
		go func() {
			httpsIp = getHttpsIp()
			log.Printf("隧道代理 | Https 请求 使用代理: %s", httpsIp)
			if clientConn == nil {
				return
			}
			defer clientConn.Close()
			destConn, err := net.DialTimeout("tcp", httpsIp, 30*time.Second)
			if err != nil {
				log.Println(err)
				return
			}
			defer destConn.Close()

			// 配置TLSClientConfig忽略证书认证
			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			destConn, err = tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", socket5Ip, tlsConfig)
			if err != nil {
				log.Println(err)
				return
			}
			defer destConn.Close()

			go io.Copy(destConn, clientConn)
			io.Copy(clientConn, destConn)
		}()
	}

}
func socket5RunTunnelProxyServer() {
	log.Println("SOCKET5 隧道代理启动 - 监听IP端口 -> ", conf.Config.Ip+":"+conf.Config.SocketTunnelPort)
	li, err := net.Listen("tcp", conf.Config.Ip+":"+conf.Config.SocketTunnelPort)
	if err != nil {
		log.Println(err)
	}
	for {
		clientConn, err := li.Accept()
		if err != nil {
			log.Panic(err)
		}
		go func() {
			socket5Ip = getSocket5Ip()
			log.Printf("隧道代理 | SOCKET5 请求 使用代理: %s", socket5Ip)
			if clientConn == nil {
				return
			}
			defer clientConn.Close()
			destConn, err := net.DialTimeout("tcp", socket5Ip, 30*time.Second)
			if err != nil {
				log.Println(err)
				return
			}
			defer destConn.Close()

			go io.Copy(destConn, clientConn)
			io.Copy(clientConn, destConn)
		}()
	}

}

// MergeArray 合并数组
func MergeArray(dest []byte, src []byte) (result []byte) {
	result = append(dest, src...)
	return
}
