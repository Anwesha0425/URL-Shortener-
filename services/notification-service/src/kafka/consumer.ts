import { Kafka, Consumer, EachMessagePayload } from 'kafkajs';
import { logger } from '../utils/logger';
import { NotificationDispatcher } from '../dispatcher';

interface ConsumerConfig {
  brokers:    string[];
  topics:     string[];
  groupId:    string;
  dispatcher: NotificationDispatcher;
}

/**
 * KafkaConsumerGroup — subscribes to multiple topics and dispatches
 * each message to the notification dispatcher.
 *
 * Retry strategy: KafkaJS built-in exponential backoff with 8 retries.
 * Dead Letter Queue: Messages that fail all retries are logged and skipped.
 */
export class KafkaConsumerGroup {
  private consumer: Consumer;
  private config:   ConsumerConfig;

  constructor(config: ConsumerConfig) {
    this.config = config;

    const kafka = new Kafka({
      clientId: 'notification-service',
      brokers:  config.brokers,
      retry: {
        initialRetryTime: 300,
        retries:          8,
      },
    });

    this.consumer = kafka.consumer({
      groupId:            config.groupId,
      sessionTimeout:     30000,
      heartbeatInterval:  3000,
    });
  }

  async start() {
    await this.consumer.connect();

    for (const topic of this.config.topics) {
      await this.consumer.subscribe({ topic, fromBeginning: false });
    }

    await this.consumer.run({
      autoCommit:              true,
      autoCommitInterval:      5000,
      eachMessage: async (payload: EachMessagePayload) => {
        await this.handleMessage(payload);
      },
    });
  }

  async stop() {
    await this.consumer.disconnect();
    logger.info('Kafka consumer disconnected');
  }

  private async handleMessage({ topic, message }: EachMessagePayload) {
    if (!message.value) return;

    try {
      const event = JSON.parse(message.value.toString());
      logger.info('Event received', { topic, eventType: event.event_type });

      await this.config.dispatcher.dispatch(topic, event);
    } catch (err) {
      // Log to DLQ in production — here we log and skip
      logger.error('Failed to process message — skipping', { topic, err });
    }
  }
}
