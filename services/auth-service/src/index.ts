import 'express-async-errors';
import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import { createServer } from 'http';

import { config } from './config';
import { logger } from './utils/logger';
import { db } from './database';
import { redis } from './cache/redis';
import { authRouter } from './routes/auth.routes';
import { apiKeyRouter } from './routes/apikey.routes';
import { metricsRouter } from './routes/metrics.routes';
import { errorHandler } from './middleware/errorHandler';
import { runMigrations } from './database/migrations';
import { startGrpcServer } from './grpc/server';

const app = express();

// ── Security middleware ───────────────────────────────────────────
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10kb' }));

// ── Health check ──────────────────────────────────────────────────
app.get('/health', async (_req, res) => {
  try {
    await db.query('SELECT 1');
    await redis.ping();
    res.json({ status: 'ok', service: 'auth-service' });
  } catch {
    res.status(503).json({ status: 'degraded' });
  }
});

// ── Routes ────────────────────────────────────────────────────────
app.use('/api/v1/auth', authRouter);
app.use('/api/v1/keys', apiKeyRouter);
app.use('/metrics', metricsRouter);

// ── Error handler ─────────────────────────────────────────────────
app.use(errorHandler);

// ── Bootstrap ─────────────────────────────────────────────────────
const server = createServer(app);

async function bootstrap() {
  try {
    await db.connect();
    logger.info('PostgreSQL connected');

    await redis.ping();
    logger.info('Redis connected');

    await runMigrations(db);
    logger.info('Migrations complete');

    // Start gRPC server (internal) on port 50051
    startGrpcServer(50051);

    server.listen(config.PORT, () => {
      logger.info(`Auth Service HTTP running on port ${config.PORT}`);
    });
  } catch (err) {
    logger.error('Bootstrap failed', { err });
    process.exit(1);
  }
}

// ── Graceful shutdown ─────────────────────────────────────────────
const shutdown = async (signal: string) => {
  logger.info(`Received ${signal}, shutting down...`);
  server.close(async () => {
    await redis.quit();
    await db.end();
    logger.info('Auth Service shut down cleanly');
    process.exit(0);
  });
};

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));

bootstrap();
