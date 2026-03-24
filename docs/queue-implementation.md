# 消息队列详细实现

**文档版本:** v1.0
**创建时间:** 2026-03-24

---

## 目录

1. [设计概述](#1-设计概述)
2. [NATS 实现](#2-nats-实现)
3. [Kafka 实现](#3-kafka-实现)
4. [内存队列](#4-内存队列)
5. [消息处理](#5-消息处理)
6. [使用示例](#6-使用示例)

---

## 1. 设计概述

### 1.1 设计目标

1. **高性能**: 高吞吐量、低延迟
2. **可靠性**: 消息不丢失、不重复
3. **可扩展**: 支持水平扩展
4. **易用性**: 简单的API和配置

### 1.2 消息模式

```
┌────────────────────────────────────────────────────────────┐
│                      Message Broker                         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐          │
│  │   Queue    │  │   Topic    │  │  Stream    │          │
│  └────────────┘  └────────────┘  └────────────┘          │
└────────┬───────────────┬───────────────┬─────────────────────┘
         │               │               │
    ┌────▼────┐    ┌───▼────┐    ┌───▼────┐
    │Consumer1│    │Consumer2│    │Consumer3│
    └─────────┘    └─────────┘    └─────────┘
```

---

## 2. NATS 实现

### 2.1 NATS 客户端

```go
// pkg/queue/nats.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSClient NATS客户端
type NATSClient struct {
	conn *nats.Conn
	js   nats.JetStreamContext
	config NATSConfig
	mu    sync.RWMutex
	stats QueueStats
}

// NATSConfig NATS配置
type NATSConfig struct {
	URL            string
	Name           string
	EnableJetStream bool
	MaxReconnects  int
	ReconnectWait   time.Duration
	Timeout         time.Duration
}

// DefaultNATSConfig 默认配置
var DefaultNATSConfig = NATSConfig{
	URL:            nats.DefaultURL,
	Name:           "online-game",
	EnableJetStream: true,
	MaxReconnects:   10,
	ReconnectWait:   2 * time.Second,
	Timeout:        10 * time.Second,
}

// NewNATSClient 创建NATS客户端
func NewNATSClient(config NATSConfig) (*NATSClient, error) {
	opts := []nats.Option{
		nats.Name(config.Name),
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectWait(config.ReconnectWait),
		nats.Timeout(config.Timeout),
	}

	conn, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	client := &NATSClient{
		conn:   conn,
		config: config,
	}

	// 启用JetStream
	if config.EnableJetStream {
		js, err := conn.JetStream()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to get JetStream context: %w", err)
		}
		client.js = js
	}

	return client, nil
}

// Publish 发布消息
func (n *NATSClient) Publish(ctx context.Context, subject string, data []byte) error {
	n.stats.Increment(StatsPublish)

	if err := n.conn.Publish(subject, data); err != nil {
		n.stats.Increment(StatsError)
		return err
	}

	return nil
}

// PublishJSON 发布JSON消息
func (n *NATSClient) PublishJSON(ctx context.Context, subject string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return n.Publish(ctx, subject, data)
}

// Request 发送请求并等待响应
func (n *NATSClient) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error) {
	n.stats.Increment(StatsRequest)

	msg, err := n.conn.Request(subject, data, timeout)
	if err != nil {
		n.stats.Increment(StatsError)
		return nil, err
	}

	n.stats.Increment(StatsResponse)
	return msg.Data, nil
}

// Subscribe 订阅消息
func (n *NATSClient) Subscribe(ctx context.Context, subject string, handler MessageHandler) (*Subscription, error) {
	n.stats.Increment(StatsSubscribe)

	sub, err := n.conn.Subscribe(subject, func(msg *nats.Msg) {
		atomic.AddInt64(&n.stats.MessagesReceived, 1)

		handler(&Message{
			Subject: msg.Subject,
			Reply:   msg.Reply,
			Data:    msg.Data,
			Headers: msg.Header,
		})
	})
	if err != nil {
		n.stats.Increment(StatsError)
		return nil, err
	}

	return &Subscription{
		sub: sub,
	}, nil
}

// QueueSubscribe 队列订阅（负载均衡）
func (n *NATSClient) QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (*Subscription, error) {
	n.stats.Increment(StatsSubscribe)

	sub, err := n.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		atomic.AddInt64(&n.stats.MessagesReceived, 1)

		handler(&Message{
			Subject: msg.Subject,
			Reply:   msg.Reply,
			Data:    msg.Data,
			Headers: msg.Header,
		})
	})
	if err != nil {
		n.stats.Increment(StatsError)
		return nil, err
	}

	return &Subscription{
		sub: sub,
	}, nil
}

// Respond 响应请求
func (n *NATSClient) Respond(msg *Message, data []byte) error {
	if msg.Reply == "" {
		return fmt.Errorf("no reply subject")
	}

	return n.conn.Publish(msg.Reply, data)
}

// Drain 排空订阅
func (n *NATSClient) Drain(sub *Subscription) error {
	return sub.sub.Drain()
}

// Unsubscribe 取消订阅
func (n *NATSClient) Unsubscribe(sub *Subscription) error {
	return sub.sub.Unsubscribe()
}

// CreateStream 创建JetStream流
func (n *NATSClient) CreateStream(ctx context.Context, cfg *nats.StreamConfig) error {
	if n.js == nil {
		return fmt.Errorf("JetStream not enabled")
	}

	_, err := n.js.AddStream(cfg)
	return err
}

// DeleteStream 删除JetStream流
func (n *NATSClient) DeleteStream(ctx context.Context, name string) error {
	if n.js == nil {
		return fmt.Errorf("JetStream not enabled")
	}

	return n.js.DeleteStream(name)
}

// StreamInfo 获取流信息
func (n *NATSClient) StreamInfo(ctx context.Context, name string) (*nats.StreamInfo, error) {
	if n.js == nil {
		return nil, fmt.Errorf("JetStream not enabled")
	}

	return n.js.StreamInfo(name)
}

// PublishMsg 发布到流（持久化）
func (n *NATSClient) PublishMsg(ctx context.Context, subject string, msg *nats.Msg) error {
	if n.js == nil {
		return fmt.Errorf("JetStream not enabled")
	}

	n.stats.Increment(StatsPublish)

	_, err := n.js.PublishMsg(subject, msg)
	if err != nil {
		n.stats.Increment(StatsError)
		return err
	}

	return nil
}

// SubscribePull 订阅拉取消息
func (n *NATSClient) SubscribePull(ctx context.Context, subject, durable string) (*PullSubscription, error) {
	if n.js == nil {
		return nil, fmt.Errorf("JetStream not enabled")
	}

	sub, err := n.js.PullSubscribe(subject, durable, nats.AckExplicit())
	if err != nil {
		return nil, err
	}

	return &PullSubscription{
		sub: sub,
	}, nil
}

// Close 关闭连接
func (n *NATSClient) Close() error {
	return n.conn.Close()
}

// Stats 返回统计信息
func (n *NATSClient) Stats() QueueStats {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.stats
}

// Subscription 订阅
type Subscription struct {
	sub *nats.Subscription
}

// PullSubscription 拉取订阅
type PullSubscription struct {
	sub *nats.Subscription
}

// Fetch 拉取消息
func (p *PullSubscription) Fetch(batch int, timeout time.Duration) ([]*nats.Msg, error) {
	return p.sub.Fetch(batch, timeout)
}

// Ack 确认消息
func (p *PullSubscription) Ack(msg *nats.Msg) error {
	return msg.Ack()
}

// Nak 否认消息
func (p *PullSubscription) Nak(msg *nats.Msg) error {
	return msg.Nak()
}

// Message 消息
type Message struct {
	Subject string
	Reply   string
	Data    []byte
	Headers nats.Header
}

// MessageHandler 消息处理器
type MessageHandler func(*Message)

// QueueStats 队列统计
type QueueStats struct {
	MessagesSent     int64
	MessagesReceived int64
	MessagesAcked    int64
	Publish          int64
	Subscribe        int64
	Request          int64
	Response         int64
	Error            int64
}

func (s *QueueStats) Increment(stat int) {
	atomic.AddInt64(&s.MessagesSent, 1)
}
```

---

## 3. Kafka 实现

### 3.1 Kafka 客户端

```go
// pkg/queue/kafka.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

// KafkaClient Kafka客户端
type KafkaClient struct {
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
	config   KafkaConfig
	mu       sync.RWMutex
	stats    QueueStats
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers         []string
	GroupID         string
	ClientID        string
	InitialOffset   string
	SessionTimeout  time.Duration
	RebalanceTimeout time.Duration
	CommitTimeout   time.Duration
}

// DefaultKafkaConfig 默认配置
var DefaultKafkaConfig = KafkaConfig{
	Brokers:         []string{"localhost:9092"},
	GroupID:         "online-game",
	ClientID:        "",
	InitialOffset:   "newest",
	SessionTimeout:   10 * time.Second,
	RebalanceTimeout: 60 * time.Second,
	CommitTimeout:   5 * time.Second,
}

// NewKafkaClient 创建Kafka客户端
func NewKafkaClient(config KafkaConfig) (*KafkaClient, error) {
	// 创建生产者
	producerConfig := sarama.NewConfig()
	producerConfig.Net.DialTimeout = 10 * time.Second
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Retry.Max = 5
	producerConfig.Producer.Retry.Backoff = 250 * time.Millisecond
	producerConfig.Metadata.Full = false

	producer, err := sarama.NewSyncProducer(producerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &KafkaClient{
		producer: producer,
		config:   config,
	}, nil
}

// Publish 发布消息
func (k *KafkaClient) Publish(ctx context.Context, topic string, key []byte, data []byte) error {
	k.stats.Increment(StatsPublish)

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		k.stats.Increment(StatsError)
		return fmt.Errorf("failed to send message: %w (partition: %d, offset: %d)", err, partition, offset)
	}

	return nil
}

// PublishJSON 发布JSON消息
func (k *KafkaClient) PublishJSON(ctx context.Context, topic string, key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return k.Publish(ctx, topic, []byte(key), data)
}

// CreateConsumer 创建消费者组
func (k *KafkaClient) CreateConsumer(topics []string, handler MessageHandler) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.consumer != nil {
		return fmt.Errorf("consumer already exists")
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Group.Session.Timeout = k.config.SessionTimeout
	config.Consumer.Group.Rebalance.Timeout = k.config.RebalanceTimeout
	config.Consumer.Return.Errors = true

	consumerGroup := sarama.NewConsumerGroup(k.config.GroupID, k.config.ClientID, config)
	if err := consumerGroup.Consume(context.Background(), topics, handler); err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	k.consumer = consumerGroup
	return nil
}

// CommitOffset 提交偏移量
func (k *KafkaClient) CommitOffset(session sarama.ConsumerGroupSession) error {
	return session.Commit()
}

// Close 关闭客户端
func (k *KafkaClient) Close() error {
	var errs []error

	if k.producer != nil {
		if err := k.producer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if k.consumer != nil {
		if err := k.consumer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// Stats 返回统计信息
func (k *KafkaClient) Stats() QueueStats {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.stats
}

// KafkaMessageHandler Kafka消息处理器
type KafkaMessageHandler struct {
	handler MessageHandler
}

// Setup 消费者组设置
func (h *KafkaMessageHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup 清理
func (h *KafkaMessageHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 消费消息
func (h *KafkaMessageHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.handler(&Message{
			Data: msg.Value,
		})

		session.MarkMessage(msg, "")
	}
	return nil
}
```

---

## 4. 内存队列

### 4.1 通道队列实现

```go
// pkg/queue/memory.go
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrQueueClosed = errors.New("queue closed")
	ErrQueueFull   = errors.New("queue full")
)

// MemoryQueue 内存队列
type MemoryQueue struct {
	name    string
	ch      chan *Message
	closed  atomic.Bool
	stats   QueueStats
	wg      sync.WaitGroup
}

// NewMemoryQueue 创建内存队列
func NewMemoryQueue(name string, capacity int) *MemoryQueue {
	return &MemoryQueue{
		name:  name,
		ch:    make(chan *Message, capacity),
	}
}

// Name 返回队列名称
func (m *MemoryQueue) Name() string {
	return m.name
}

// Push 推送消息
func (m *MemoryQueue) Push(ctx context.Context, msg *Message) error {
	if m.closed.Load() {
		return ErrQueueClosed
	}

	m.stats.Increment(StatsPublish)

	select {
	case m.ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pop 弹出消息
func (m *MemoryQueue) Pop(ctx context.Context) (*Message, error) {
	if m.closed.Load() {
		return nil, ErrQueueClosed
	}

	select {
	case msg := <-m.ch:
		m.stats.Increment(StatsMessagesReceived)
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TryPop 尝试弹出（非阻塞）
func (m *MemoryQueue) TryPop() (*Message, error) {
	select {
	case msg := <-m.ch:
		m.stats.Increment(StatsMessagesReceived)
		return msg, nil
	default:
		return nil, nil
	}
}

// Publish 发布消息
func (m *MemoryQueue) Publish(ctx context.Context, subject string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	msg := &Message{
		Subject: subject,
		Data:    data,
	}

	return m.Push(ctx, msg)
}

// Subscribe 订阅消息
func (m *MemoryQueue) Subscribe(ctx context.Context, handler MessageHandler) error {
	m.wg.Add(1)
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := m.Pop(ctx)
		if err != nil {
			if err == ErrQueueClosed {
				return nil
			}
			continue
		}

		if err := handler(msg); err != nil {
			// 处理错误
		}
	}
}

// Size 返回队列大小
func (m *MemoryQueue) Size() int {
	return len(m.ch)
}

// Close 关闭队列
func (m *MemoryQueue) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		close(m.ch)
	}
	return nil
}

// Stats 返回统计信息
func (m *MemoryQueue) Stats() QueueStats {
	return m.stats
}
```

---

## 5. 消息处理

### 5.1 消息处理器

```go
// pkg/queue/processor.go
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/your-org/online-game/pkg/logger"
)

// Processor 消息处理器
type Processor struct {
	queue    Queue
	handler  MessageHandler
	workers  int
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stats    ProcessorStats
}

// ProcessorStats 处理器统计
type ProcessorStats struct {
	MessagesProcessed int64
	MessagesFailed    int64
	MessagesRetried   int64
	ActiveWorkers     int32
}

// NewProcessor 创建处理器
func NewProcessor(queue Queue, handler MessageHandler, workers int) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		queue:   queue,
		handler: handler,
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动处理器
func (p *Processor) Start() error {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return nil
}

// Stop 停止处理器
func (p *Processor) Stop() error {
	p.cancel()
	p.wg.Wait()
	return nil
}

// worker 工作协程
func (p *Processor) worker() {
	defer p.wg.Done()

	atomic.AddInt32(&p.stats.ActiveWorkers, 1)
	defer atomic.AddInt32(&p.stats.ActiveWorkers, -1)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// 获取消息
		msg, err := p.queue.Pop(p.ctx)
		if err != nil {
			if err == ErrQueueClosed {
				return
			}
			continue
		}

		// 处理消息
		if err := p.processMessage(msg); err != nil {
			atomic.AddInt64(&p.stats.MessagesFailed, 1)

			// 重试逻辑
			if shouldRetry(msg) {
				p.retryMessage(msg)
			}
		} else {
			atomic.AddInt64(&p.stats.MessagesProcessed, 1)
		}
	}
}

// processMessage 处理消息
func (p *Processor) processMessage(msg *Message) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Message processor panic", "msg", msg, "panic", r)
		}
	}()

	// 设置超时
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	return p.handler(ctx, msg)
}

// retryMessage 重试消息
func (p *Processor) retryMessage(msg *Message) {
	atomic.AddInt64(&p.stats.MessagesRetried, 1)

	// 增加重试计数
	retries := getRetries(msg)
	setRetries(msg, retries+1)

	// 延迟重试
	delay := calculateBackoff(retries)
	time.Sleep(delay)

	// 重新入队
	_ = p.queue.Push(context.Background(), msg)
}

// Stats 返回统计信息
func (p *Processor) Stats() ProcessorStats {
	return p.stats
}

// shouldRetry 判断是否应该重试
func shouldRetry(msg *Message) bool {
	retries := getRetries(msg)
	return retries < 3
}

// getRetries 获取重试次数
func getRetries(msg *Message) int {
	if msg.Headers == nil {
		return 0
	}
	if v, ok := msg.Headers["x-retries"]; ok {
		if n, ok := v.(int); ok {
			return n
		}
		if n, ok := v.(int64); ok {
			return int(n)
		}
		if s, ok := v.(string); ok {
			var n int
			fmt.Sscanf(s, "%d", &n)
			return n
		}
	}
	return 0
}

// setRetries 设置重试次数
func setRetries(msg *Message, count int) {
	if msg.Headers == nil {
		msg.Headers = make(map[string]interface{})
	}
	msg.Headers["x-retries"] = count
}

// calculateBackoff 计算退避延迟
func calculateBackoff(retries int) time.Duration {
	// 指数退避
	base := 100 * time.Millisecond
	delay := base * time.Duration(1<<uint(retries))
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}
```

---

## 6. 使用示例

### 6.1 NATS 使用示例

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/online-game/pkg/queue"
)

func main() {
	// 创建NATS客户端
	client, err := queue.NewNATSClient(queue.DefaultNATSConfig)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	// 订阅消息
	sub, err := client.Subscribe(ctx, "game.>", func(msg *queue.Message) {
		fmt.Printf("Received message on %s: %s\n", msg.Subject, msg.Data)
	})
	if err != nil {
		panic(err)
	}
	defer client.Unsubscribe(sub)

	// 发布消息
	err = client.PublishJSON(ctx, "game.player.joined", map[string]interface{}{
		"player_id": "12345",
		"game_id":   "67890",
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second)
}
```

### 6.2 消息处理示例

```go
package main

import (
	"context"
	"log"

	"github.com/your-org/online-game/pkg/queue"
)

func main() {
	// 创建队列
	queue := queue.NewMemoryQueue("events", 1000)

	// 创建处理器
	handler := func(ctx context.Context, msg *queue.Message) error {
		log.Printf("Processing message: %s", msg.Subject)
		// 处理消息
		return nil
	}

	processor := queue.NewProcessor(queue, handler, 4)
	processor.Start()
	defer processor.Stop()

	// 发布消息
	_ = queue.Publish(context.Background(), "user.login", map[string]interface{}{
		"user_id": "12345",
	})

	time.Sleep(time.Second)
}
```
