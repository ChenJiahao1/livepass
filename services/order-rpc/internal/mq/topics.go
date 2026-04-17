package mq

import "livepass/services/order-rpc/internal/config"

const (
	DefaultOrderCreateTopic         = "ticketing.attempt.command.v1"
	DefaultOrderCreateConsumerGroup = "livepass-ticketing-attempt"
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
