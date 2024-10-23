package main

import (
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
	ResponseTime string    // 响应时间
	Score        int       `gorm:"default:10"` // 评分
	Anonymity    string    // 匿名级别
	LastChecked  time.Time `gorm:"autoCreateTime"` // 最后验证时间
	Isp          string    // IP提供商
	IsWorking    bool      `gorm:"default:true"`   // 是否可用
	FailCount    int       `gorm:"default:0"`      // 连续失败次数
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
		updates["FailCount"] = 0
	} else {
		updates["Score"] = gorm.Expr("Score - 3")
		db.Model(&ProxyIp{}).
			Where("ip_address = ? AND port = ? AND protocol = ? AND Score < 0", pi.IPAddress, pi.Port, pi.Protocol).
			UpdateColumn("FailCount", gorm.Expr("fail_count + 1"))
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
	db.Where("Score <= 0 AND fail_count >= 3").
		Delete(&ProxyIp{})

	log.Println("失效代理已清理")

	// 重新加载代理池
	loadProxyPool()
}
