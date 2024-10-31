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
	err = db.AutoMigrate(&ProxyIp{})
	if err != nil {
		log.Printf("自动迁移数据库结构失败:%v", err)
		return
	}
}

func getProxyPool(isWorking *bool) []ProxyIp {
	var proxyPool []ProxyIp
	query := db.Model(&ProxyIp{})
	if isWorking != nil {
		query = query.Where("is_working = ?", *isWorking)
	}
	if err := query.Find(&proxyPool).Error; err != nil {
		log.Printf("加载代理池失败 (is_working = %v): %v", isWorking, err)
		return []ProxyIp{}
	}
	return proxyPool
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
}

// 删除指定代理
func delProxy(ipAddress string, port string, protocol string) error {
	// 尝试删除指定条件的代理
	result := db.Where("ip_address = ? AND port = ? AND protocol = ?", ipAddress, port, protocol).Delete(&ProxyIp{})

	// 检查是否删除成功
	if result.Error != nil {
		log.Printf("删除代理失败：%v", result.Error)
		return result.Error
	}

	// 检查受影响的行数
	if result.RowsAffected == 0 {
		log.Println("未找到符合条件的代理")
		return fmt.Errorf("no proxy found with ip_address=%s, port=%s, protocol=%s", ipAddress, port, protocol)
	}

	log.Printf("已删除代理: %s:%s (%s)", ipAddress, port, protocol)
	return nil
}

// 获取指定协议的代理IP
func getProxyIp(protocol string, anonymity string, country string, count int) ([]ProxyIp, error) {
	mu.Lock()
	defer mu.Unlock()

	// 创建一个 ProxyIp 变量来存储查询结果
	var proxies []ProxyIp
	// 查询代理池中可用的代理
	query := db.Model(&ProxyIp{})
	// 根据传入的参数值进行筛选
	if protocol != "all" {
		query = query.Where("protocol = ?", protocol)
	}
	if anonymity != "all" {
		query = query.Where("anonymity = ?", anonymity)
	}
	if country != "all" {
		query = query.Where("protocol = ?", country)
	}
	query = query.
		Where("is_working = ?", true).
		Where("response_time < ?", conf.Config.TunnelTime)
	// 按上次使用时间升序，优先选择最近未使用的代理
	err := query.Order("last_used ASC").
		Limit(count).
		Find(&proxies).Error
	if err != nil || len(proxies) == 0 {
		return proxies, err
	}

	// 更新每个代理的 `last_used` 时间
	for _, proxy := range proxies {
		db.Model(&ProxyIp{}).
			Where("ip_address = ? AND port = ? AND protocol = ?", proxy.IPAddress, proxy.Port, proxy.Protocol).
			Update("last_used", time.Now())
	}
	return proxies, nil
}

func getHttpIp() string {
	proxies, err := getProxyIp("HTTP", "all", "all", 1)
	if err != nil || len(proxies) == 0 {
		log.Println("Failed to get HTTP proxy:", err)
		return ""
	}
	return proxies[0].IPAddress + ":" + proxies[0].Port
}
func getHttpsIp() string {
	proxies, err := getProxyIp("HTTPS", "all", "all", 1)
	if err != nil || len(proxies) == 0 {
		log.Println("Failed to get HTTP proxy:", err)
		return ""
	}
	return proxies[0].IPAddress + ":" + proxies[0].Port
}

func getSocket5Ip() string {
	proxies, err := getProxyIp("SOCKET5", "all", "all", 1)
	if err != nil || len(proxies) == 0 {
		log.Println("Failed to get HTTP proxy:", err)
		return ""
	}
	return proxies[0].IPAddress + ":" + proxies[0].Port
}
func getConnectIp() string {
	proxies, err := getProxyIp("CONNECT", "all", "all", 1)
	if err != nil || len(proxies) == 0 {
		log.Println("Failed to get HTTP proxy:", err)
		return ""
	}
	return proxies[0].IPAddress + ":" + proxies[0].Port
}

func getProxyCount(protocol string, isWorking *bool) int {
	mu.Lock()
	defer mu.Unlock()
	var count int64
	query := db.Model(&ProxyIp{})

	// 根据传入的参数值进行筛选
	if protocol != "all" {
		query = query.Where("protocol = ?", protocol)
	}
	if isWorking != nil {
		query = query.Where("is_working = ?", *isWorking)
	}

	// 统计符合条件的代理数量
	if err := query.Count(&count).Error; err != nil {
		log.Printf("统计代理数量失败: %v", err)
		return 0
	}
	return int(count)
}

// 获取代理池统计信息
func getProxyPoolStats(isWorking bool) Home {
	home := Home{
		Type:        make(map[string]int),
		Anonymity:   make(map[string]int),
		Country:     make(map[string]int),
		Source:      make(map[string]int),
		TunnelProxy: make(map[string]string),
	}

	// 获取总计
	var total int64
	db.Model(&ProxyIp{}).Where("is_working = ?", isWorking).Count(&total)
	home.Sum = int(total)

	// 统计各字段的分布
	type Result struct {
		Key   string
		Count int
	}

	// 使用 GORM 分别统计 Protocol、Anonymity、Country、Source 分布情况
	var typeResults, anonymityResults, countryResults, sourceResults []Result

	db.Model(&ProxyIp{}).
		Select("protocol as key, COUNT(*) as count").
		Where("is_working = ?", isWorking).
		Group("protocol").
		Scan(&typeResults)

	db.Model(&ProxyIp{}).
		Select("anonymity as key, COUNT(*) as count").
		Where("is_working = ?", isWorking).
		Group("anonymity").
		Scan(&anonymityResults)

	db.Model(&ProxyIp{}).
		Select("country as key, COUNT(*) as count").
		Where("is_working = ?", isWorking).
		Group("country").
		Scan(&countryResults)

	db.Model(&ProxyIp{}).
		Select("source as key, COUNT(*) as count").
		Where("is_working = ?", isWorking).
		Group("source").
		Scan(&sourceResults)

	// 将结果存储到 Home 的相应字段中
	for _, result := range typeResults {
		home.Type[result.Key] = result.Count
	}
	for _, result := range anonymityResults {
		home.Anonymity[result.Key] = result.Count
	}
	for _, result := range countryResults {
		home.Country[result.Key] = result.Count
	}
	for _, result := range sourceResults {
		home.Source[result.Key] = result.Count
	}

	return home
}
