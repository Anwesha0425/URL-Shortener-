import 'express-async-errors';
import express from 'express';
import { createServer } from 'http';
import { logger } from './utils/logger';
import { KafkaConsumerGroup } from './kafka/consumer';
import { NotificationDispatcher } from './dispatcher';

const app = express();
app.use(express.json());

app.get('/health', (_req, res) => res.json({ status: 'ok', service: 'notification-service' }));

const server = createServer(app);
const PORT   = process.env.PORT || '8005';
const BROKERS = process.env.KAFKA_BROKERS || 'localhost:9092';

async function bootstrap() {
  // Kafka consumer subscribes to notification-relevant topics
  const consumer = new KafkaConsumerGroup({
    brokers:  BROKERS.split(','),
    topics:   ['url.created', 'url.expired', 'analytics.milestone'],
    groupId:  'notification-service',
    dispatcher: new NotificationDispatcher(),
  });

  await consumer.start();
  logger.info('Kafka consumer started');

  server.listen(PORT, () => {
    logger.info(`Notification Service running on port ${PORT}`);
  });

  const shutdown = async () => {
    await consumer.stop();
    server.close(() => process.exit(0));
  };

  process.on('SIGTERM', shutdown);
  process.on('SIGINT',  shutdown);
}

bootstrap().catch((err) => {
  logger.error('Bootstrap failed', { err });
  process.exit(1);
});
