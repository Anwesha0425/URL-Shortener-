import { Router } from 'express';
import { register, collectDefaultMetrics, Counter, Histogram, Gauge } from 'prom-client';

collectDefaultMetrics({ register });

// Custom metrics
export const httpRequestsTotal = new Counter({
  name:       'auth_http_requests_total',
  help:       'Total HTTP requests to auth service',
  labelNames: ['method', 'route', 'status'],
  registers:  [register],
});

export const rateLimitHits = new Counter({
  name:      'auth_rate_limit_hits_total',
  help:      'Total rate limit rejections',
  labelNames: ['tier'],
  registers:  [register],
});

export const tokenIssuedTotal = new Counter({
  name:      'auth_tokens_issued_total',
  help:      'Total JWT tokens issued',
  labelNames: ['type'],
  registers:  [register],
});

export const activeUsers = new Gauge({
  name:     'auth_active_users',
  help:     'Current number of active users',
  registers: [register],
});

export const metricsRouter = Router();

metricsRouter.get('/', async (_req, res) => {
  res.set('Content-Type', register.contentType);
  res.end(await register.metrics());
});
