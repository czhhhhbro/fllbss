package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Asset 玩家资产结构
type Asset struct {
	UserID  string  `json:"userId"`
	Balance float64 `json:"balance"`
}

// User 用户登录结构
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 全局存储 (生产环境请替换为数据库)
var (
	assets  = map[string]float64{
		"user1":  1250.0,
		"user2":  980.0,
		"user3":  2100.0,
		"user4":  550.0,
		"user5":  1800.0,
		"user6":  320.0,
		"user7":  2500.0,
		"user8":  780.0,
		"user9":  1450.0,
		"user10": 420.0,
	}
	users = map[string]string{"admin": "123456"} // 初始管理员账号
	mu    sync.Mutex                           // 并发锁

	// 记录用户每日奖励领取情况 (key: username_YYYY-MM-DD)
	dailyRewards = map[string]string{}
)

// 跨域中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 获取所有玩家资产
func getAssets(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	var assetList []Asset
	for uid, bal := range assets {
		assetList = append(assetList, Asset{UserID: uid, Balance: bal})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"assets":  assetList,
	})
}

// 登录接口
func login(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, `{"success":false,"message":"参数错误"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if pwd, ok := users[user.Username]; ok && pwd == user.Password {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"user":    user,
		})
	} else {
		http.Error(w, `{"success":false,"message":"账号或密码错误"}`, http.StatusUnauthorized)
	}
}

// 注册接口
func register(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, `{"success":false,"message":"参数错误"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := users[user.Username]; exists {
		http.Error(w, `{"success":false,"message":"用户名已存在"}`, http.StatusConflict)
		return
	}

	users[user.Username] = user.Password
	assets[user.Username] = 1000.0 // 新用户初始资产

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "注册成功",
	})
}

// 转账接口
func transfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"success":false,"message":"参数错误"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if assets[req.From] < req.Amount {
		http.Error(w, `{"success":false,"message":"余额不足"}`, http.StatusBadRequest)
		return
	}

	assets[req.From] -= req.Amount
	assets[req.To] += req.Amount

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "转账成功",
	})
}

// 每日签到领取100法罗兰币
func dailyReward(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"success":false,"message":"参数错误"}`, http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	today := time.Now().Format("2006-01-02")
	rewardKey := req.Username + "_" + today

	if _, exists := dailyRewards[rewardKey]; exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "今日奖励已领取",
		})
		return
	}

	assets[req.Username] += 100.0
	dailyRewards[rewardKey] = time.Now().Format("2006-01-02 15:04:05")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "成功领取100法罗兰币！",
		"newBalance": assets[req.Username],
	})
}

// 定时任务：每天凌晨0点，给所有在线用户自动增加100 FLO
func startDailyRewardTask() {
	for {
		now := time.Now()
		// 计算明天凌晨0点
		tomorrow := now.AddDate(0, 0, 1)
		midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
		duration := midnight.Sub(now)

		println("定时任务等待中，下次发币时间:", midnight.Format("2006-01-02 15:04:05"))

		select {
		case <-time.After(duration):
			mu.Lock()
			// 给所有用户发币
			for user := range users {
				assets[user] += 100.0
			}
			// 重置每日领取记录 (可选，如果想每天都能领)
			dailyRewards = map[string]string{}
			mu.Unlock()
			println("定时任务执行：已为所有用户增加100 FLO")
		}
	}
}

func main() {
	// 启动定时发币任务
	go startDailyRewardTask()

	// 注册路由
	http.HandleFunc("/api/assets", getAssets)
	http.HandleFunc("/api/login", login)
	http.HandleFunc("/api/register", register)
	http.HandleFunc("/api/transfer", transfer)
	http.HandleFunc("/api/daily-reward", dailyReward)

	// 静态文件服务 (访问根路径直接打开index.html)
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/", fs)

	// 启动服务器
	println("服务器启动在 http://localhost:8080")
	server := &http.Server{
		Addr:    ":8080",
		Handler: corsMiddleware(http.DefaultServeMux),
	}
	server.ListenAndServe()
}
