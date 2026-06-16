import { Redis } from 'ioredis';
import { config } from '../config';
import { logger } from '../utils/logger';

export const redis = new Redis({
  host: config.REDIS_HOST,
  port: config.REDIS_PORT,
  maxRetriesPerRequest: 3,
  lazyConnect: true,
  enableReadyCheck: true,
});

redis.on('connect',       () => logger.info('Redis: connected'));
redis.on('error',   (err) => logger.error('Redis error', { err }));
redis.on('reconnecting',  () => logger.warn('Redis: reconnecting...'));
