#!/usr/bin/env bash
#
# Creates the Kafka topics from docs/TZ.md §8.3 with their planned
# partition counts, plus a DLQ per topic.
#
# Why this exists rather than relying on auto-creation: the compose file
# has KAFKA_AUTO_CREATE_TOPICS_ENABLE=true for convenience, but a topic
# created that way gets the broker's *default* partition count (1), and
# §8.3's ordering guarantee ("per key only") assumes the planned counts.
# A single-partition qrmenu.order.v1 would appear to work in dev and then
# behave differently in production, which is the worst kind of difference.
# The relay therefore does not ask kafka-go to auto-create topics either —
# see pkg/outbox.New.
#
# Idempotent: re-running is a no-op for topics that already exist.

set -euo pipefail

CONTAINER="${KAFKA_CONTAINER:-qrmenu-kafka-1}"
BOOTSTRAP="${KAFKA_BOOTSTRAP:-localhost:9092}"
RETENTION_MS=$((7 * 24 * 60 * 60 * 1000))     # 7d, per §8.3
DLQ_RETENTION_MS=$((30 * 24 * 60 * 60 * 1000)) # 30d, per §8.3

# topic:partitions:retention_ms
TOPICS=(
  "qrmenu.order.v1:6:$RETENTION_MS"
  "qrmenu.service_request.v1:3:$RETENTION_MS"
  "qrmenu.catalog.v1:3:$RETENTION_MS"
  "qrmenu.order.v1.dlq:1:$DLQ_RETENTION_MS"
  "qrmenu.service_request.v1.dlq:1:$DLQ_RETENTION_MS"
  "qrmenu.catalog.v1.dlq:1:$DLQ_RETENTION_MS"
)

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "kafka container '$CONTAINER' is not running — run 'make infra-up' first" >&2
  exit 1
fi

for spec in "${TOPICS[@]}"; do
  IFS=: read -r topic partitions retention <<<"$spec"
  docker exec "$CONTAINER" /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server "$BOOTSTRAP" \
    --create --if-not-exists \
    --topic "$topic" \
    --partitions "$partitions" \
    --replication-factor 1 \
    --config "retention.ms=$retention" >/dev/null
  echo "==> $topic (partitions=$partitions, retention=${retention}ms)"
done

echo
docker exec "$CONTAINER" /opt/kafka/bin/kafka-topics.sh --bootstrap-server "$BOOTSTRAP" --list | grep '^qrmenu\.'
