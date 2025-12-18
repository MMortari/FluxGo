package fluxgo

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type KafkaProducer struct {
	sarama.SyncProducer
}
type KafkaConsumerGroup struct {
	closed bool
	sarama.ConsumerGroup
}
type Kafka struct {
	opts        KafkaOptions
	producerCon KafkaProducer
	consumerCon KafkaConsumerGroup

	consumers []Consumer
	topics    []string

	apm *Apm
}

type KafkaProducerOptions struct {
	Acks sarama.RequiredAcks
}
type KafkaConsumerOptions struct {
	AutoCommitEnable bool
	GroupId          string
}
type KafkaOptions struct {
	Brokers  []string
	UsesTls  bool
	Producer KafkaProducerOptions
	Consumer KafkaConsumerOptions
}

func (f *FluxGo) AddKafka(opt KafkaOptions) *FluxGo {
	f.AddDependency(func(apm *Apm) *Kafka {
		kafka := Kafka{
			opts: opt,
			apm:  apm,
		}

		producer, err := setupProducer(opt)
		if err != nil {
			f.logger.Panicf("Failed to create producer: %s", err.Error())
		}

		consumer, err := setupConsumer(opt)
		if err != nil {
			f.logger.Panicf("Failed to create consumer: %s", err.Error())
		}

		kafka.producerCon = producer
		kafka.consumerCon = consumer

		return &kafka
	})
	f.AddInvoke(func(lc fx.Lifecycle, kafka *Kafka) error {
		if err := kafka.CheckConnection(); err != nil {
			return err
		}

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				kafka.RunConsumers()
				f.log("KAFKA", "Consumer connected")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := kafka.consumerCon.Close(); err != nil {
					return err
				}
				f.log("KAFKA", "Consumer disconnected")
				return nil
			},
		})
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				kafka.consumerCon.closed = true
				if err := kafka.producerCon.Close(); err != nil {
					return err
				}
				f.log("KAFKA", "Producer disconnected")
				return nil
			},
		})
		return nil
	})

	return f
}

func setupProducer(opt KafkaOptions) (KafkaProducer, error) {
	prodConfig := sarama.NewConfig()
	prodConfig.Producer.Return.Successes = true
	prodConfig.Producer.RequiredAcks = opt.Producer.Acks

	tlsConfig := tls.Config{}
	prodConfig.Net.TLS.Enable = opt.UsesTls
	prodConfig.Net.TLS.Config = &tlsConfig

	producer, err := sarama.NewSyncProducer(opt.Brokers, prodConfig)

	if err != nil {
		return KafkaProducer{}, err
	}

	return KafkaProducer{producer}, nil
}
func setupConsumer(opt KafkaOptions) (KafkaConsumerGroup, error) {
	configConsumer := sarama.NewConfig()
	configConsumer.Consumer.Offsets.AutoCommit.Enable = opt.Consumer.AutoCommitEnable

	groupId := opt.Consumer.GroupId

	consumerGroup, err := sarama.NewConsumerGroup(opt.Brokers, groupId, configConsumer)

	if err != nil {
		return KafkaConsumerGroup{}, err
	}

	return KafkaConsumerGroup{ConsumerGroup: consumerGroup}, nil
}

func (k *Kafka) CheckConnection() error {
	configConsumer := sarama.NewConfig()

	admin, err := sarama.NewClusterAdmin(k.opts.Brokers, configConsumer)
	if err != nil {
		return err
	}

	if err := admin.Close(); err != nil {
		return err
	}

	return nil
}

type KafkaHandler func(ctx context.Context, data []byte) error
type Consumer struct {
	topic   string
	handler KafkaHandler
}

func (k *Kafka) AddConsumer(topic string, handler KafkaHandler) error {
	k.topics = append(k.topics, topic)
	k.consumers = append(k.consumers, Consumer{
		topic:   topic,
		handler: handler,
	})

	return nil
}

func (k *Kafka) RunConsumers() {
	han := ConsumerGroup{
		consumers: k.consumers,
		client:    k,
	}

	log.Println("Processing topics", k.topics)

	go func() {
		for !k.consumerCon.closed {
			ctx := context.Background()
			err := k.consumerCon.Consume(ctx, k.topics, han)

			if err != nil {
				log.Panicf("Error from consumer: err %v", err)
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
			}
			if ctx.Err() != nil {
				log.Panicf("Error from consumer: ctx %v", err)
				return
			}
		}
	}()
}

type ConsumerGroup struct {
	consumers []Consumer
	client    *Kafka
}

func (ConsumerGroup) Setup(_ sarama.ConsumerGroupSession) error {
	// log.Printf("[KAFKA] Consumer group is being rebalanced")
	return nil
}

func (ConsumerGroup) Cleanup(_ sarama.ConsumerGroupSession) error {
	// log.Printf("[KAFKA] Re-balancing will happen soon, current session will end")
	return nil
}

func (h ConsumerGroup) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.handleMessage(msg, session)
	}
	return nil
}

func (h *ConsumerGroup) handleMessage(msg *sarama.ConsumerMessage, session sarama.ConsumerGroupSession) {
	poolCtx, span := h.client.apm.StartSpan(context.Background(), fmt.Sprintf("pool %s", msg.Topic), trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	if consumer := h.getHandler(msg.Topic); consumer != nil {
		ctx, processSpan := h.client.apm.StartSpan(context.Background(), fmt.Sprintf("process %s", msg.Topic), trace.WithSpanKind(trace.SpanKindServer), trace.WithLinks(trace.LinkFromContext(poolCtx)))
		defer processSpan.End()

		if err := consumer.handler(ctx, msg.Value); err == nil {
			session.MarkMessage(msg, "")
		} else {
			processSpan.SetStatus(codes.Error, err.Error())
			processSpan.RecordError(err)
		}
	}
}
func (h *ConsumerGroup) getHandler(topic string) *Consumer {
	for _, item := range h.consumers {
		if item.topic == topic {
			return &item
		}
	}

	return nil
}

func (k *Kafka) ProduceMessageJson(ctx context.Context, topic string, data interface{}, key *string) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return k.produceMessage(ctx, topic, sarama.StringEncoder(b), key)
}
func (k *Kafka) produceMessage(ctx context.Context, topic string, data sarama.Encoder, key *string) error {
	_, span := k.apm.StartSpan(ctx, fmt.Sprintf("send %s", topic), trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", topic),
		attribute.String("messaging.operation.name", "send"),
		attribute.String("messaging.operation.type", "send"),
	))
	defer span.End()

	if key != nil {
		span.SetAttributeString("messaging.kafka.message.key", *key)
	}

	message := &sarama.ProducerMessage{
		Topic: topic,
		Value: data,
	}

	if key != nil {
		message.Key = sarama.StringEncoder(*key)
	}

	partition, offset, err := k.producerCon.SendMessage(message)

	if err != nil {
		span.SetError(err)
		return err
	}

	span.SetAttributes(
		attribute.String("messaging.destination.partition.id", fmt.Sprint(partition)),
		attribute.Int64("messaging.kafka.offset", offset),
	)

	return nil
}
