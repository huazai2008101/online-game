# 故障排查手册

**文档版本:** v1.0
**创建时间:** 2026-03-24
**适用范围:** online-game 平台

---

## 目录

1. [快速诊断](#快速诊断)
2. [服务故障](#服务故障)
3. [数据库问题](#数据库问题)
4. [缓存问题](#缓存问题)
5. [消息队列问题](#消息队列问题)
6. [网络问题](#网络问题)
7. [性能问题](#性能问题)
8. [内存泄漏](#内存泄漏)
9. [死锁问题](#死锁问题)
10. [部署问题](#部署问题)

---

## 快速诊断

### 健康检查端点

```bash
# API Gateway
curl http://localhost:8080/health

# WebSocket Gateway
curl http://localhost:8081/health

# 各个服务
curl http://localhost:8001/health  # game-service
curl http://localhost:8002/health  # user-service
curl http://localhost:8003/health  # payment-service
curl http://localhost:8004/health  # match-service
```

### 查看服务状态

```bash
# 使用 systemctl (systemd 环境)
sudo systemctl status online-game-*
sudo journalctl -u online-game-gateway -f

# 使用 Docker
docker ps
docker logs -f <container_id>

# 使用 Kubernetes
kubectl get pods -l app=online-game
kubectl logs -f deployment/gateway
kubectl describe pod <pod_name>
```

### 检查端口监听

```bash
# 检查服务端口是否在监听
netstat -tuln | grep -E '8080|8081|8001|8002|8003|8004'

# 使用 ss 替代 netstat
ss -tuln | grep -E '8080|8081|8001|8002|8003|8004'

# 检查端口占用
lsof -i :8080
```

---

## 服务故障

### 问题: 服务启动失败

**症状:**
- 服务无法启动
- 日志显示启动错误
- 端口已被占用

**排查步骤:**

1. 检查配置文件:
```bash
# 验证配置文件格式
cat /etc/online-game/config.yaml | yq eval

# 检查必需的环境变量
env | grep ONLINE_GAME_
```

2. 检查端口占用:
```bash
# 查找占用端口的进程
sudo lsof -i :8080
sudo netstat -tulpn | grep 8080

# 杀死占用进程
sudo kill -9 <PID>
```

3. 检查依赖服务:
```bash
# 检查 PostgreSQL
pg_isready -h localhost -p 5432

# 检查 Redis
redis-cli ping

# 检查 NATS
echo "INFO" | nc localhost 4222
```

4. 查看详细日志:
```bash
# 增加日志级别
export LOG_LEVEL=debug

# 重启服务
sudo systemctl restart online-game-gateway

# 查看完整日志
sudo journalctl -u online-game-gateway -n 100 --no-pager
```

### 问题: 服务频繁重启

**症状:**
- 服务反复崩溃重启
- 健康检查失败
- OOM (Out of Memory) 错误

**排查步骤:**

1. 检查内存使用:
```bash
# 查看进程内存
ps aux | grep online-game

# 使用 pmap 查看内存映射
sudo pmap <PID>

# 检查系统内存
free -h
vmstat 1 10
```

2. 检查 goroutine 泄漏:
```bash
# 启用 pprof 端点
curl http://localhost:8080/debug/pprof/goroutine?debug=1

# 使用 go tool pprof 分析
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

3. 检查死锁:
```bash
# 查看所有 goroutine 堆栈
curl http://localhost:8080/debug/pprof/goroutine?debug=2

# 搜索死锁信号
curl http://localhost:8080/debug/pprof/ | grep deadlock
```

### 问题: 服务响应缓慢

**症状:**
- API 响应时间长
- 超时错误频繁
- CPU 使用率高

**排查步骤:**

1. CPU 性能分析:
```bash
# 采集 CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof

# 分析 CPU profile
go tool pprof cpu.prof

# 生成火焰图
go tool pprof -http=:8081 cpu.prof
```

2. 检查慢查询:
```sql
-- PostgreSQL 慢查询
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 启用慢查询日志
ALTER SYSTEM SET log_min_duration_statement = 1000;
```

3. 网络延迟测试:
```bash
# 测试数据库延迟
pgbench -h localhost -p 5432 -c 10 -j 2 -T 10 online_game

# 测试 Redis 延迟
redis-cli --latency

# 测试网络延迟
ping -c 100 localhost
```

---

## 数据库问题

### 问题: 连接池耗尽

**症状:**
- "connection pool exhausted" 错误
- 新请求被拒绝
- 数据库连接数达到上限

**排查步骤:**

1. 检查连接数:
```sql
-- 查看当前连接数
SELECT count(*) FROM pg_stat_activity;

-- 查看最大连接数
SHOW max_connections;

-- 查看每个数据库的连接数
SELECT datname, count(*)
FROM pg_stat_activity
GROUP BY datname;
```

2. 检查空闲连接:
```sql
-- 查看长时间空闲的连接
SELECT pid, usename, datname, state, query_start, state_change
FROM pg_stat_activity
WHERE state = 'idle'
AND state_change < now() - interval '5 minutes';
```

3. 调整连接池配置:
```yaml
# config.yaml
database:
  max_open_connections: 100
  max_idle_connections: 10
  connection_max_lifetime: 1h
  connection_max_idle_time: 10m
```

### 问题: 慢查询

**症状:**
- 查询响应时间长
- 数据库 CPU 使用率高
- 锁等待

**排查步骤:**

1. 启用查询日志:
```sql
-- 启用慢查询日志
ALTER SYSTEM SET log_min_duration_statement = 1000;
SELECT pg_reload_conf();

-- 查看当前设置
SHOW log_min_duration_statement;
```

2. 分析查询计划:
```sql
-- 分析慢查询
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT * FROM users WHERE email = 'user@example.com';

-- 使用 pg_stat_statements
SELECT query, calls, total_exec_time, mean_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

3. 检查索引:
```sql
-- 查找缺失的索引
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'public'
ORDER BY n_distinct DESC;

-- 查看索引使用情况
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read
FROM pg_stat_user_indexes
ORDER BY idx_scan ASC;
```

### 问题: 数据库锁

**症状:**
- 事务被阻塞
- 更新/删除操作挂起
- "lock wait timeout" 错误

**排查步骤:**

1. 检查锁状态:
```sql
-- 查看当前锁
SELECT pid, relname, mode, granted
FROM pg_locks l
JOIN pg_class c ON l.relation = c.oid
WHERE NOT granted;

-- 查看阻塞关系
SELECT blocked_locks.pid AS blocked_pid,
       blocking_activity.usename AS blocking_user,
       blocking_activity.query AS blocking_query,
       blocked_activity.usename AS blocked_user,
       blocked_activity.query AS blocked_query
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

2. 终止长时间运行的查询:
```sql
-- 查看长时间运行的事务
SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

-- 终止查询
SELECT pg_terminate_backend(<PID>);
```

---

## 缓存问题

### 问题: 缓存穿透

**症状:**
- 大量请求未命中缓存
- 数据库负载过高
- 响应变慢

**解决方案:**

1. 布隆过滤器:
```go
// 使用布隆过滤器预检查
func (s *Service) GetUser(id string) (*User, error) {
    if !s.bloomFilter MightContain(id) {
        return nil, ErrUserNotFound
    }

    // 检查缓存
    if user, ok := s.cache.Get(id); ok {
        return user, nil
    }

    // 查询数据库
    user, err := s.repo.FindByID(id)
    if err != nil {
        return nil, err
    }

    s.cache.Set(id, user, 5*time.Minute)
    return user, nil
}
```

2. 空值缓存:
```go
func (s *Service) GetUser(id string) (*User, error) {
    // 检查缓存
    cached, ok := s.cache.Get(id)
    if ok {
        if cached == nil {
            return nil, ErrUserNotFound
        }
        return cached.(*User), nil
    }

    user, err := s.repo.FindByID(id)
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            // 缓存空值，防止穿透
            s.cache.Set(id, nil, 1*time.Minute)
            return nil, err
        }
        return nil, err
    }

    s.cache.Set(id, user, 5*time.Minute)
    return user, nil
}
```

### 问题: 缓存雪崩

**症状:**
- 大量缓存同时过期
- 数据库瞬时负载激增
- 服务响应变慢

**解决方案:**

1. 缓存过期时间加随机值:
```go
func (s *Service) SetCache(key string, value interface{}) {
    // 基础过期时间 + 随机时间
    baseExp := 5 * time.Minute
    randomExp := time.Duration(rand.Intn(60)) * time.Second
    s.cache.Set(key, value, baseExp+randomExp)
}
```

2. 缓存预热:
```go
func (s *Service) WarmupCache(ctx context.Context) error {
    // 启动时预加载热点数据
    hotUsers, err := s.repo.FindHotUsers(ctx, 1000)
    if err != nil {
        return err
    }

    for _, user := range hotUsers {
        s.cache.Set(user.ID, user, 10*time.Minute)
    }

    return nil
}
```

3. 多级缓存:
```go
type MultiLevelCache struct {
    l1 *sync.Map          // 本地缓存
    l2 *redis.Client      // Redis 缓存
}

func (m *MultiLevelCache) Get(key string) (interface{}, bool) {
    // L1: 本地缓存
    if v, ok := m.l1.Load(key); ok {
        return v, true
    }

    // L2: Redis 缓存
    v, err := m.l2.Get(ctx, key).Result()
    if err == nil {
        m.l1.Store(key, v)
        return v, true
    }

    return nil, false
}
```

---

## 消息队列问题

### 问题: 消息堆积

**症状:**
- 消息队列深度持续增长
- 消费延迟增加
- 内存使用上升

**排查步骤:**

1. 检查队列状态:
```bash
# NATS 检查
nats-server -js info
nbox info

# 查看流状态
nats str info <stream_name>

# 查看消费者
nats con info <consumer_name>
```

2. 检查消费者健康:
```bash
# 查看消费者状态
curl http://localhost:8080/metrics | grep consumer

# 查看消息处理速率
curl http://localhost:8080/metrics | grep message_processing_rate
```

3. 调整消费者配置:
```yaml
# 增加消费者数量
consumers:
  count: 10
  parallelism: 5

# 调整预取数量
prefetch_count: 100
```

### 问题: 消息丢失

**症状:**
- 消息发送后未被处理
- 消息计数不匹配
- 数据不一致

**排查步骤:**

1. 检查确认机制:
```go
// 确保消息被正确确认
func (c *Consumer) ProcessMessage(msg *nats.Msg) error {
    defer msg.Ack() // 确保消息被确认

    // 处理消息
    if err := c.handle(msg); err != nil {
        msg.Nak() // 消息处理失败，重新入队
        return err
    }

    return nil
}
```

2. 启用持久化:
```go
// 创建持久化流
js, _ := nc.JetStream()
js.AddStream(&nats.StreamConfig{
    Name:     "events",
    Subjects: []string{"events.>"},
    Storage:  nats.FileStorage,  // 持久化存储
    Replicas: 3,                  // 副本数
})
```

---

## 网络问题

### 问题: 连接超时

**症状:**
- 请求超时
- "connection timeout" 错误
- 服务间通信失败

**排查步骤:**

1. 检查网络连通性:
```bash
# 测试端口连通性
telnet localhost 8080
nc -zv localhost 8080

# 测试 DNS 解析
nslookup api.example.com
dig api.example.com

# 追踪路由
traceroute api.example.com
mtr api.example.com
```

2. 检查防火墙:
```bash
# 查看防火墙规则
sudo iptables -L -n
sudo firewall-cmd --list-all

# 检查 SELinux
getenforce
sudo ausearch -m avc -ts recent
```

3. 调整超时配置:
```yaml
http:
  timeout:
    dial: 10s
    response_header: 5s
    request: 30s
    idle: 90s
```

### 问题: WebSocket 连接断开

**症状:**
- WebSocket 频繁断开
- 客户端不断重连
- "connection reset by peer" 错误

**排查步骤:**

1. 检查代理配置:
```nginx
# Nginx WebSocket 配置
location /ws {
    proxy_pass http://ws-gateway:8081;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

2. 启用心跳:
```go
// WebSocket 心跳
func (c *Conn) StartHeartbeat(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        case <-c.done:
            return
        }
    }
}
```

---

## 性能问题

### 问题: GC 压力大

**症状:**
- 频繁的 GC 暂停
- CPU 使用率高
- 延迟抖动

**排查步骤:**

1. 查看 GC 统计:
```bash
# 查看 GC 统计
curl http://localhost:8080/debug/pprof/heap?debug=1

# 启用 GC 日志
export GODEBUG=gctrace=1
./gateway
```

2. 优化内存分配:
```go
// 使用对象池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func process() {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    // 使用 buf
}
```

3. 调整 GC 参数:
```go
func init() {
    // 调整 GC 目标百分比
    debug.SetGCPercent(100)

    // 设置内存限制 (Go 1.19+)
    debug.SetMemoryLimit(1 << 30) // 1GB
}
```

---

## 内存泄漏

### 问题: 内存持续增长

**症状:**
- 进程内存持续增长
- OOM 崩溃
- GC 无法回收内存

**排查步骤:**

1. 采集 heap profile:
```bash
# 采集当前堆内存
curl http://localhost:8080/debug/pprof/heap > heap.prof

# 分析堆内存
go tool pprof heap.prof

# 查看分配最多的函数
(pprof) top
(pprof) list <function_name>
```

2. 比较差异:
```bash
# 采集两个时间点的 heap
curl http://localhost:8080/debug/pprof/heap > heap1.prof
# 等待一段时间
curl http://localhost:8080/debug/pprof/heap > heap2.prof

# 比较差异
go tool pprof -base heap1.prof heap2.prof
```

3. 常见泄漏点检查:
```go
// 检查 goroutine 泄漏
go func() {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop() // 确保停止 ticker

    for range ticker.C {
        // 处理
    }
}()

// 检查未关闭的连接
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close() // 确保关闭

// 检查 context 取消
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel() // 确保取消
```

---

## 死锁问题

### 问题: 程序挂起

**症状:**
- 程序无响应
- CPU 使用率为 0
- goroutine 数量不变

**排查步骤:**

1. 检测死锁:
```bash
# 获取所有 goroutine 堆栈
curl http://localhost:8080/debug/pprof/goroutine?debug=2 > goroutine.txt

# 搜索锁等待
grep -i "lock" goroutine.txt
grep -i "chan receive" goroutine.txt
```

2. 使用竞态检测:
```bash
# 运行时启用竞态检测
go run -race main.go

# 查看竞态报告
```

3. 避免死锁模式:
```go
// 错误: 锁顺序不一致导致死锁
// 正确: 定义锁顺序
type SafeMap struct {
    mu   sync.Mutex
    data map[string]interface{}
}

var globalLock sync.Mutex // 全局锁，用于协调

func (m *SafeMap) Set(key string, value interface{}) {
    globalLock.Lock()
    m.mu.Lock()
    defer m.mu.Unlock()
    defer globalLock.Unlock()

    m.data[key] = value
}
```

---

## 部署问题

### 问题: Pod 无法启动

**症状:**
- Pod 状态为 CrashLoopBackOff
- Pod 状态为 ImagePullBackOff
- Pod 状态为 Pending

**排查步骤:**

1. 查看 Pod 状态:
```bash
kubectl get pods
kubectl describe pod <pod_name>
kubectl logs <pod_name>
```

2. 常见错误处理:
```bash
# 镜像拉取失败
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<username> \
  --docker-password=<password>

# 资源不足
kubectl top nodes
kubectl top pods

# 配置错误
kubectl get configmap
kubectl get secret
```

### 问题: 滚动更新失败

**症状:**
- 新版本 Pod 无法就绪
- 更新回滚
- 服务中断

**排查步骤:**

1. 检查就绪探针:
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

2. 配置滚动更新策略:
```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```

---

## 应急处理

### 服务回滚

```bash
# Kubernetes 回滚
kubectl rollout undo deployment/gateway

# Docker 回滚
docker-compose down
docker-compose up -d --scale gateway=3

# Git 回滚
git revert HEAD
git push origin main
```

### 紧急扩容

```bash
# Kubernetes 扩容
kubectl scale deployment/gateway --replicas=10

# Docker 扩容
docker-compose up -d --scale gateway=10
```

### 流量切换

```bash
# 切换到备用服务
kubectl patch svc gateway -p '{"spec":{"selector":{"version":"v2"}}}'

# 熔断降级
curl -X POST http://localhost:8080/admin/circuit-breaker/activate
```
