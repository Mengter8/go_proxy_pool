package main

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"sync"
	"time"
)

var db *gorm.DB
var mu sync.Mutex

type ProxyIp struct {
	IPAddress    string    `gorm:"not null;uniqueIndex:unique_proxy"` // 代理IP地址
	Port         string    `gorm:"not null;uniqueIndex:unique_proxy"` // 代理端口
	Protocol     string    `gorm:"not null;uniqueIndex:unique_proxy"` // 代理协议
	countryCode  string    `gorm:"not null"`                          // 国家代码
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

// 更新代理的评分、最后验证时间及工作状态
func updateProxyRecord(pi *ProxyIp) {
	mu.Lock()
	defer mu.Unlock()
	updates := map[string]interface{}{
		"last_checked":  time.Now(),
		"response_time": pi.ResponseTime,
		"is_working":    pi.IsWorking,
	}

	// 根据验证结果调整评分
	if pi.IsWorking {
		updates["score"] = gorm.Expr("score + 1")
	} else {
		updates["score"] = gorm.Expr("score - 2")
	}

	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip_address"}, {Name: "port"}, {Name: "protocol"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(pi).Error

	if err != nil {
		log.Println("数据插入或更新失败：", err)
		return
	}

	log.Println("数据插入或更新成功")
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
	mu.Lock()
	defer mu.Unlock()

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
