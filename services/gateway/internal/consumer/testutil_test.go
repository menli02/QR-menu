package consumer

import "github.com/segmentio/kafka-go"

// kafkaMessage wraps a raw envelope the way the reader would deliver it.
func kafkaMessage(value string) kafka.Message {
	return kafka.Message{Topic: "qrmenu.order.v1", Value: []byte(value)}
}
