package mq

import "damai-go/services/order-rpc/internal/config"

const (
	DefaultOrderCreateTopic         = "order.create.command.v1"
	DefaultOrderCreateConsumerGroup = "damai-go-order-create"
)

func OrderCreateTopic(cfg config.KafkaConfig) string {
	if cfg.TopicOrderCreate != "" {
		return cfg.TopicOrderCreate
	}

	return DefaultOrderCreateTopic
}

func OrderCreateConsumerGroup(cfg config.KafkaConfig) string {
	if cfg.ConsumerGroup != "" {
		return cfg.ConsumerGroup
	}

	return DefaultOrderCreateConsumerGroup
}
