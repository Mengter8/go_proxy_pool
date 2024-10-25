package main

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"time"
)

var db *gorm.DB

type ProxyIp struct {
	IPAddress    string    `gorm:"not null"` // 代理IP地址
	Port         string    `gorm:"not null"` // 代理端口
	Protocol     string    `gorm:"not null"` // 代理协议
	countryCode  string    `gorm:"not null"` // 国家代码
	Country      string    // 代理所在国家
	ResponseTime int64     // 响应时间
	Score        int       `gorm:"default:10"` // 评分
	Anonymity    string    // 匿名级别
	LastChecked  time.Time `gorm:"autoCreateTime"` // 最后验证时间
	LastUsed     time.Time `gorm:"autoCreateTime"` // 最后使用时间
	Isp          string    // IP提供商
	IsWorking    bool      `gorm:"default:true"`   // 是否可用
	CreatedAt    time.Time `gorm:"autoCreateTime"` // 添加时间
	Source       string    `gorm:"not null"`       // 代理数据源
	Province     string    // 省份
	City         string    // 城市
}

// 初始化SQLite数据库
func initSqlite() {
	var err error
	db, err = gorm.Open(sqlite.Open("db.sqlite"), &gorm.Config{})
	if err != nil {
		log.Println("连接数据库失败：", err)
	} else {
		log.Println("连接数据库成功")
	}

	// 自动迁移数据库结构
	db.AutoMigrate(&ProxyIp{})

	// 初始化代理池
	loadProxyPool()
}

// 加载代理池
func loadProxyPool() {
	if err := db.Find(&ProxyPool).Error; err != nil {
		log.Println("加载代理池失败：", err)
		panic("failed to load proxy pool")
	}
}

// 创建代理URL记录
func CreateProxyRecord(data *ProxyIp) {
	if !proxyExists(data.IPAddress, data.Port, data.Protocol) {
		err := db.Create(data).Error
		if err != nil {
			log.Println("插入数据失败：", err)
			return
		}
		log.Println("插入数据成功")
	}
}

// 更新代理的评分、最后验证时间及工作状态
func updateProxyStatus(pi *ProxyIp, IsWorking bool) {
	updates := map[string]interface{}{
		"LastChecked":  time.Now(),
		"ResponseTime": pi.ResponseTime,
		"IsWorking":    pi.IsWorking,
	}

	// 根据验证结果更新评分和失败计数
	if IsWorking {
		updates["Score"] = gorm.Expr("Score + 1")
	} else {
		updates["Score"] = gorm.Expr("Score - 2")
	}

	// 更新数据库中的记录
	db.Model(&ProxyIp{}).
		Where("ip_address = ? AND port = ? AND protocol = ?", pi.IPAddress, pi.Port, pi.Protocol).
		Updates(updates)
}

// 检查代理是否已存在
func proxyExists(IPAddress string, Port string, Protocol string) bool {
	result := db.First(&ProxyIp{}, "ip_address = ? AND port = ? AND protocol = ?", IPAddress, Port, Protocol)
	return result.RowsAffected > 0
}

// 清理失效代理
func cleanInvalidProxies() {
	db.Where("Score <= 0").
		Delete(&ProxyIp{})

	log.Println("失效代理已清理")

	// 重新加载代理池
	loadProxyPool()
}

// 获取指定协议的代理IP
func getProxyIp(protocol string) string {
	lock2.Lock()
	defer lock2.Unlock()

	// 创建一个 ProxyIp 变量来存储查询结果
	var proxy ProxyIp

	// 查询代理池中可用的代理
	err := db.Where("protocol = ? AND is_working = ? AND response_time < ?", protocol, true, conf.Config.TunnelTime).
		Order("last_used ASC"). // 按上次使用时间升序，优先选择最近未使用的代理
		First(&proxy).Error

	if err != nil {
		log.Println("No available proxy found:", err)
		return ""
	}

	// 更新代理的上次使用时间
	err = db.Model(&ProxyIp{}).
		Where("ip_address = ? AND port = ? AND protocol = ?", proxy.IPAddress, proxy.Port, proxy.Protocol).
		Update("last_used", time.Now()).Error
	if err != nil {
		log.Println("Failed to update last_used:", err)
		return ""
	}

	// 返回 IP 地址和端口
	return fmt.Sprintf("%s:%s", proxy.IPAddress, proxy.Port)
}

func getHttpIp() string {
	return getProxyIp("HTTP")
}

func getHttpsIp() string {
	return getProxyIp("HTTPS")
}

func getSocket5Ip() string {
	return getProxyIp("SOCKET5")
}
func getConnectIp() string {
	return getProxyIp("CONNECT")
}
