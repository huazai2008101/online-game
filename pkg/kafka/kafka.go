package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

var (
	ErrProducerClosed    = errors.New("producer is closed")
	ErrConsumerClosed    = errors.New("consumer is closed")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrNoMessageReceived = errors.New("no message received")
)

// Message represents a Kafka message
type Message struct {
	Topic string
	Key   string
	Value []byte
	Headers map[string]string
	Time  time.Time
}

// ProducerConfig defines producer configuration
type ProducerConfig struct {
	Brokers          []string
	Topic            string
	MaxMessageBytes  int
	Timeout          time.Duration
	FlushFrequency   time.Duration
	FlushMessages    int
	FlushBytes       int
	Compression      sarama.CompressionCodec
	AckRequired      sarama.RequiredAcks
	MaxRetries       int
	Backoff          time.Duration
}

// DefaultProducerConfig returns default producer configuration
func DefaultProducerConfig() *ProducerConfig {
	return &ProducerConfig{
		Brokers:         []string{"localhost:9092"},
		MaxMessageBytes: 1000000,
		Timeout:         10 * time.Second,
		FlushFrequency:  100 * time.Millisecond,
		FlushMessages:   100,
		FlushBytes:      1048576, // 1MB
		Compression:     sarama.CompressionSnappy,
		AckRequired:     sarama.WaitForAll,
		MaxRetries:      3,
		Backoff:         100 * time.Millisecond,
	}
}

// Producer handles message publishing to Kafka
type Producer struct {
	producer    sarama.SyncProducer
	asyncProd   sarama.AsyncProducer
	config      *ProducerConfig
	mu          sync.RWMutex
	closed      atomic.Bool
	stats       *ProducerStats
	errorChan   chan error
}

// ProducerStats tracks producer statistics
type ProducerStats struct {
	MessagesSent    atomic.Int64
	BytesSent       atomic.Int64
	SendErrors      atomic.Int64
	Retries         atomic.Int64
	MaxLatency      atomic.Int64 // nanoseconds
	TotalLatency    atomic.Int64 // nanoseconds
}

// NewProducer creates a new Kafka producer
func NewProducer(config *ProducerConfig) (*Producer, error) {
	if config == nil {
		config = DefaultProducerConfig()
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.MaxMessageBytes = config.MaxMessageBytes
	saramaConfig.Producer.Timeout = config.Timeout
	saramaConfig.Producer.Compression = config.Compression
	saramaConfig.Producer.RequiredAcks = config.AckRequired
	saramaConfig.Producer.Retry.Max = config.MaxRetries
	saramaConfig.Producer.Retry.Backoff = config.Backoff

	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync producer: %w", err)
	}

	p := &Producer{
		producer:  producer,
		config:    config,
		stats:     &ProducerStats{},
		errorChan: make(chan error, 100),
	}

	return p, nil
}

// SendMessage sends a message to Kafka
func (p *Producer) SendMessage(ctx context.Context, topic, key string, value interface{}) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	start := time.Now()

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.stats.SendErrors.Add(1)
		return fmt.Errorf("failed to send message: %w", err)
	}

	latency := time.Since(start)
	p.updateLatency(latency)
	p.stats.MessagesSent.Add(1)
	p.stats.BytesSent.Add(int64(len(data)))

	_ = partition
	_ = offset

	return nil
}

// SendBatch sends multiple messages in batch
func (p *Producer) SendBatch(ctx context.Context, messages []*Message) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}

	if len(messages) == 0 {
		return nil
	}

	// Group messages by topic
	topicMessages := make(map[string][]*sarama.ProducerMessage)

	for _, msg := range messages {
		data, err := json.Marshal(msg.Value)
		if err != nil {
			continue
		}

		kafkaMsg := &sarama.ProducerMessage{
			Topic: msg.Topic,
			Key:   sarama.StringEncoder(msg.Key),
			Value: sarama.ByteEncoder(data),
			Timestamp: msg.Time,
		}

		// Add headers
		if len(msg.Headers) > 0 {
			headers := make([]sarama.RecordHeader, 0, len(msg.Headers))
			for k, v := range msg.Headers {
				headers = append(headers, sarama.RecordHeader{
					Key:   []byte(k),
					Value: []byte(v),
				})
			}
			kafkaMsg.Headers = headers
		}

		topicMessages[msg.Topic] = append(topicMessages[msg.Topic], kafkaMsg)
	}

	start := time.Now()

	// Send messages for each topic
	for topic, msgs := range topicMessages {
		for _, msg := range msgs {
			_, _, err := p.producer.SendMessage(msg)
			if err != nil {
				p.stats.SendErrors.Add(1)
				return fmt.Errorf("failed to send message to topic %s: %w", topic, err)
			}

			p.stats.MessagesSent.Add(1)
			if len(msg.Value.(sarama.ByteEncoder)) > 0 {
				p.stats.BytesSent.Add(int64(len(msg.Value.(sarama.ByteEncoder))))
			}
		}
	}

	latency := time.Since(start)
	p.updateLatency(latency)

	return nil
}

// updateLatency updates latency statistics
func (p *Producer) updateLatency(latency time.Duration) {
	nanos := latency.Nanoseconds()
	p.stats.TotalLatency.Add(nanos)

	for {
		max := p.stats.MaxLatency.Load()
		if nanos <= max {
			break
		}
		if p.stats.MaxLatency.CompareAndSwap(max, nanos) {
			break
		}
	}
}

// GetStats returns producer statistics
func (p *Producer) GetStats() *ProducerStats {
	return p.stats
}

// Errors returns the error channel
func (p *Producer) Errors() <-chan error {
	return p.errorChan
}

// Close closes the producer
func (p *Producer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.producer != nil {
		return p.producer.Close()
	}

	return nil
}

// ConsumerConfig defines consumer configuration
type ConsumerConfig struct {
	Brokers        []string
	GroupID        string
	Topics         []string
	InitialOffset  int64 // sarama.OffsetOldest or sarama.OffsetNewest
	Heartbeat      time.Duration
	Timeout        time.Duration
	RebalanceTimeout time.Duration
	RebalanceStrategy sarama.BalanceStrategy
}

// DefaultConsumerConfig returns default consumer configuration
func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		Brokers:         []string{"localhost:9092"},
		GroupID:         "game-consumer-group",
		Topics:          []string{},
		InitialOffset:   sarama.OffsetNewest,
		Heartbeat:       3 * time.Second,
		Timeout:         10 * time.Second,
		RebalanceTimeout: 60 * time.Second,
		RebalanceStrategy: sarama.BalanceStrategyRoundRobin,
	}
}

// MessageHandler handles consumed messages
type MessageHandler func(ctx context.Context, msg *Message) error

// Consumer handles message consumption from Kafka
type Consumer struct {
	consumer    sarama.ConsumerGroup
	config      *ConsumerConfig
	handler     MessageHandler
	mu          sync.RWMutex
	closed      atomic.Bool
	stats       *ConsumerStats
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// ConsumerStats tracks consumer statistics
type ConsumerStats struct {
	MessagesReceived atomic.Int64
	BytesReceived    atomic.Int64
	ProcessErrors    atomic.Int64
	ProcessingTime   atomic.Int64 // nanoseconds
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(config *ConsumerConfig, handler MessageHandler) (*Consumer, error) {
	if config == nil {
		config = DefaultConsumerConfig()
	}

	if handler == nil {
		return nil, errors.New("handler cannot be nil")
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Consumer.Group.Rebalance.Timeout = config.RebalanceTimeout
	saramaConfig.Consumer.Group.Rebalance.Strategy = config.RebalanceStrategy
	saramaConfig.Consumer.Group.Heartbeat.Interval = config.Heartbeat
	saramaConfig.Consumer.Offsets.Initial = config.InitialOffset

	consumer, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		consumer: consumer,
		config:   config,
		handler:  handler,
		stats:    &ConsumerStats{},
		ctx:      ctx,
		cancel:   cancel,
	}

	return c, nil
}

// Start starts consuming messages
func (c *Consumer) Start() error {
	if c.closed.Load() {
		return ErrConsumerClosed
	}

	c.wg.Add(1)
	go c.consume()

	return nil
}

// consume is the main consumption loop
func (c *Consumer) consume() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if err := c.consumer.Consume(c.ctx, c.config.Topics, c); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				// Log error but continue consuming
				c.stats.ProcessErrors.Add(1)
			}
		}
	}
}

// Setup is called at the beginning of a new session
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called at the end of a session
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from claimed partitions
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
			select {
			case msg, ok := <-claim.Messages():
				if !ok {
					return nil
				}

				start := time.Now()

				// Convert to our Message type
				message := &Message{
					Topic:     msg.Topic,
					Key:       string(msg.Key),
					Value:     msg.Value,
					Time:      msg.Timestamp,
					Headers:   make(map[string]string),
				}

				for _, h := range msg.Headers {
					message.Headers[string(h.Key)] = string(h.Value)
				}

				// Call handler
				if err := c.handler(c.ctx, message); err != nil {
					c.stats.ProcessErrors.Add(1)
				} else {
					// Mark message as processed only if handler succeeded
					session.MarkMessage(msg, "")
				}

				// Update stats
				processingTime := time.Since(start)
				c.stats.ProcessingTime.Add(processingTime.Nanoseconds())
				c.stats.MessagesReceived.Add(1)
				c.stats.BytesReceived.Add(int64(len(msg.Value)))
			}
		}
	}
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() *ConsumerStats {
	return c.stats
}

// Close closes the consumer
func (c *Consumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	c.cancel()
	c.wg.Wait()

	if c.consumer != nil {
		return c.consumer.Close()
	}

	return nil
}

// AdminClient provides Kafka admin operations
type AdminClient struct {
	client sarama.ClusterAdmin
}

// NewAdminClient creates a new Kafka admin client
func NewAdminClient(brokers []string) (*AdminClient, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0 // Use appropriate version

	client, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}

	return &AdminClient{client: client}, nil
}

// CreateTopic creates a new topic
func (a *AdminClient) CreateTopic(topic string, partitions int32, replicationFactor int16) error {
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     partitions,
		ReplicationFactor: replicationFactor,
	}

	return a.client.CreateTopic(topic, topicDetail, false)
}

// DeleteTopic deletes a topic
func (a *AdminClient) DeleteTopic(topic string) error {
	return a.client.DeleteTopic(topic)
}

// ListTopics lists all topics
func (a *AdminClient) ListTopics() (map[string]sarama.TopicDetail, error) {
	return a.client.ListTopics()
}

// DescribeTopic describes a topic
func (a *AdminClient) DescribeTopic(topic string) (sarama.TopicDetail, error) {
	topics, err := a.client.ListTopics()
	if err != nil {
		return sarama.TopicDetail{}, err
	}

	detail, ok := topics[topic]
	if !ok {
		return sarama.TopicDetail{}, fmt.Errorf("topic not found: %s", topic)
	}

	return detail, nil
}

// Close closes the admin client
func (a *AdminClient) Close() error {
	return a.client.Close()
}

// EventBus provides a simple event bus interface using Kafka
type EventBus struct {
	producer *Producer
	consumer *Consumer
	topics   map[string]bool
	mu       sync.RWMutex
}

// NewEventBus creates a new event bus
func NewEventBus(producer *Producer, consumer *Consumer) *EventBus {
	return &EventBus{
		producer: producer,
		consumer: consumer,
		topics:   make(map[string]bool),
	}
}

// Publish publishes an event
func (eb *EventBus) Publish(ctx context.Context, topic, eventType string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := &Message{
		Topic: topic,
		Key:   eventType,
		Value: data,
		Time:  time.Now(),
		Headers: map[string]string{
			"event_type": eventType,
		},
	}

	return eb.producer.SendBatch(ctx, []*Message{msg})
}

// Subscribe subscribes to events from a topic
func (eb *EventBus) Subscribe(topics []string, handler MessageHandler) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for _, topic := range topics {
		eb.topics[topic] = true
	}

	// Update consumer topics
	eb.consumer.config.Topics = topics

	// Start consumer if not already running
	if !eb.consumer.closed.Load() {
		return eb.consumer.Start()
	}

	return nil
}

// Close closes the event bus
func (eb *EventBus) Close() error {
	var errs []error

	if eb.producer != nil {
		if err := eb.producer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if eb.consumer != nil {
		if err := eb.consumer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing event bus: %v", errs)
	}

	return nil
}
