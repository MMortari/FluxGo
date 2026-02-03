package fluxgo

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type Kafka struct {
	apm *Apm

	producer      KafkaProduce
	consumerGroup KafkaConsumerGroup

	consumers []Consumer

	consumerIsRunning bool
	opts              KafkaOptions
}

type KafkaProduce sarama.SyncProducer
type KafkaConsumerGroup sarama.ConsumerGroup

type Consumer struct {
	topic   string
	handler MessageHandler
}
type MessageHandler func(ctx context.Context, data []byte) error

type KafkaConsumerOptions struct {
	GroupId    string
	AutoCommit bool
}
type KafkaProducerOptions struct {
	Acks sarama.RequiredAcks
}
type KafkaAuth struct {
	TlsEnabled bool
}
type KafkaOptions struct {
	Brokers []string
	Auth    KafkaAuth

	Consumer *KafkaConsumerOptions
	Producer *KafkaProducerOptions
}

type ConsumerInterface interface {
	HandleMessage(ctx context.Context, data []byte) error
}

func (f *FluxGo) AddKafka(data KafkaOptions) *FluxGo {
	f.AddDependency(func(apm *Apm) *Kafka {
		kafka := Kafka{
			apm:  apm,
			opts: data,
		}

		if data.Producer != nil {
			producer, err := setupProducer(data)
			if err != nil {
				log.Printf("Failed to create producer: %s\n", err)
				panic("Failed to create producer")
			}

			kafka.producer = producer
		}

		if data.Consumer != nil {
			consumer, err := setupConsumer(data)
			if err != nil {
				log.Printf("Failed to create consumer: %s\n", err)
				panic("Failed to create consumer")
			}

			kafka.consumerGroup = consumer
		}

		return &kafka
	})

	f.AddInvoke(func(lc fx.Lifecycle, kafka *Kafka) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				f.log("KAFKA", "Connected")
				return kafka.start()
			},
			OnStop: func(ctx context.Context) error {
				f.log("KAFKA", "Disconnected")
				return kafka.stop()
			},
		})
		return nil
	})

	return f
}

func (k *Kafka) start() error {
	if len(k.consumers) == 0 {
		return nil
	}

	var topics []string

	for _, item := range k.consumers {
		topics = append(topics, string(item.topic))
	}

	han := ConsumerGroup{
		apm:       k.apm,
		consumers: k.consumers,
	}
	k.consumerIsRunning = true

	go func() {
		for k.consumerIsRunning {
			ctx := context.Background()

			if err := k.consumerGroup.Consume(ctx, topics, han); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
			}
			if err := ctx.Err(); err != nil {
				return
			}
		}
	}()

	return nil
}
func (k *Kafka) stop() error {
	if err := k.consumerGroup.Close(); err != nil {
		return err
	}
	k.consumerIsRunning = false

	if err := k.producer.Close(); err != nil {
		return err
	}

	return nil
}
func (k *Kafka) AddConsumer(topic string, handler MessageHandler) error {
	k.consumers = append(k.consumers, Consumer{
		topic:   topic,
		handler: handler,
	})
	return nil
}
func (k *Kafka) ProduceMessageJson(ctx context.Context, topic string, data interface{}, key *string) error {
	var parseData sarama.Encoder

	if b, err := json.Marshal(data); err != nil {
		parseData = sarama.StringEncoder((make([]byte, 0)))
	} else {
		parseData = sarama.StringEncoder(b)
	}

	return k.ProduceMessage(ctx, topic, parseData, key)
}
func (k *Kafka) ProduceMessage(ctx context.Context, topic string, data sarama.Encoder, key *string) error {
	var spanAtt = []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", string(topic)),
		attribute.String("messaging.operation.name", "send"),
		attribute.String("messaging.operation.type", "send"),
		attribute.String("server.address", k.opts.Brokers[0]),
	}
	if key != nil {
		spanAtt = append(spanAtt, attribute.String("messaging.kafka.message.key", *key))
	}

	_, span := k.apm.StartSpan(ctx, "event/kafka/produceMessage", trace.WithAttributes(spanAtt...))
	defer span.End()

	message := &sarama.ProducerMessage{
		Topic: string(topic),
		Value: data,
	}

	if key != nil {
		message.Key = sarama.StringEncoder(*key)
	}

	if _, _, err := k.producer.SendMessage(message); err != nil {
		span.SetError(err)
		return err
	}

	return nil
}

func setupAuth(config *sarama.Config, options KafkaOptions) {
	tlsConfig := tls.Config{}
	config.Net.TLS.Enable = options.Auth.TlsEnabled
	config.Net.TLS.Config = &tlsConfig
}
func setupProducer(options KafkaOptions) (KafkaProduce, error) {
	prodConfig := sarama.NewConfig()

	prodConfig.Producer.Return.Successes = true
	prodConfig.Producer.RequiredAcks = options.Producer.Acks

	setupAuth(prodConfig, options)

	return sarama.NewSyncProducer(options.Brokers, prodConfig)
}
func setupConsumer(options KafkaOptions) (KafkaConsumerGroup, error) {
	configConsumer := sarama.NewConfig()
	configConsumer.Consumer.Offsets.AutoCommit.Enable = options.Consumer.AutoCommit

	setupAuth(configConsumer, options)

	groupId := options.Consumer.GroupId

	consumerGroup, err := sarama.NewConsumerGroup(options.Brokers, groupId, configConsumer)

	if err != nil {
		return consumerGroup, err
	}

	return consumerGroup, nil
}

type ConsumerGroup struct {
	apm       *Apm
	consumers []Consumer
}

func (ConsumerGroup) Setup(_ sarama.ConsumerGroupSession) error {
	log.Printf("[KAFKA] Consumer group is being rebalanced")
	return nil
}
func (ConsumerGroup) Cleanup(_ sarama.ConsumerGroupSession) error {
	log.Printf("[KAFKA] Re-balancing will happen soon, current session will end")
	return nil
}
func (h ConsumerGroup) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.handleMessage(msg, session)
	}
	return nil
}
func (h *ConsumerGroup) handleMessage(msg *sarama.ConsumerMessage, session sarama.ConsumerGroupSession) {
	ctx, span := h.apm.StartSpan(context.Background(), msg.Topic, trace.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.Int64("messaging.kafka.offset", msg.Offset),
		attribute.String("messaging.kafka.message.key", string(msg.Key)),
		attribute.String("messaging.destination.name", msg.Topic),
		attribute.String("messaging.operation.name", "process"),
		attribute.String("messaging.operation.type", "process"),
	))
	defer span.End()

	if consumer := h.getHandler(msg.Topic); consumer != nil {
		if err := consumer.handler(ctx, msg.Value); err == nil {
			session.MarkMessage(msg, "")

			span.SetStatus(codes.Ok, "Success")
		} else {
			span.SetStatus(codes.Error, "Failure")
			span.RecordError(err)
		}
	}
}
func (h *ConsumerGroup) getHandler(topic string) *Consumer {
	for _, item := range h.consumers {
		if string(item.topic) == topic {
			return &item
		}
	}

	return nil
}
