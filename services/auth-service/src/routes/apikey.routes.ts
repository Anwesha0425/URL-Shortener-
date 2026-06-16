import { Router, Request, Response } from 'express';
import { v4 as uuidv4 } from 'uuid';
import { db } from '../database';
import { checkRateLimit } from '../cache/rateLimiter';
import { logger } from '../utils/logger';

export const apiKeyRouter = Router();

/**
 * POST /api/v1/keys/check-rate-limit
 * Called by API Gateway to check if a request is within rate limits.
 * Returns 200 with remaining tokens, or 429 with retry-after.
 */
apiKeyRouter.post('/check-rate-limit', async (req: Request, res: Response) => {
  const { identifier, tier = 'free' } = req.body as { identifier: string; tier: string };

  if (!identifier) {
    return res.status(400).json({ error: 'identifier required' });
  }

  const result = await checkRateLimit(identifier, tier as 'free' | 'pro' | 'enterprise');

  if (!result.allowed) {
    res.setHeader('X-RateLimit-Limit',     result.limit);
    res.setHeader('X-RateLimit-Remaining', 0);
    res.setHeader('Retry-After',           Math.ceil((result.retryAfterMs || 1000) / 1000));
    return res.status(429).json({
      error:        'Rate limit exceeded',
      retryAfterMs: result.retryAfterMs,
    });
  }

  res.setHeader('X-RateLimit-Limit',     result.limit);
  res.setHeader('X-RateLimit-Remaining', result.remaining);
  return res.status(200).json({ allowed: true, remaining: result.remaining });
});

/**
 * POST /api/v1/keys
 * Generate a new API key for a user.
 */
apiKeyRouter.post('/', async (req: Request, res: Response) => {
  const { userId, name } = req.body as { userId: string; name: string };

  if (!userId || !name) {
    return res.status(400).json({ error: 'userId and name required' });
  }

  const apiKey = `uk_live_${uuidv4().replace(/-/g, '')}`;

  await db.query(
    `INSERT INTO api_keys (user_id, key_hash, name)
     VALUES ($1, $2, $3)`,
    [userId, apiKey, name],
  );

  logger.info('API key created', { userId, name });
  res.status(201).json({
    key:       apiKey,
    name,
    warning:   'Store this key securely. It will not be shown again.',
    createdAt: new Date().toISOString(),
  });
});

/**
 * GET /api/v1/keys/:userId
 * List all API keys for a user (without exposing full key).
 */
apiKeyRouter.get('/:userId', async (req: Request, res: Response) => {
  const { userId } = req.params;
  const result = await db.query(
    `SELECT id, name, LEFT(key_hash, 12) || '...' as key_preview, created_at, last_used_at
     FROM api_keys WHERE user_id = $1 AND is_active = true ORDER BY created_at DESC`,
    [userId],
  );
  res.json({ keys: result.rows });
});

/**
 * DELETE /api/v1/keys/:keyId
 * Revoke an API key.
 */
apiKeyRouter.delete('/:keyId', async (req: Request, res: Response) => {
  const { keyId } = req.params;
  await db.query('UPDATE api_keys SET is_active = false WHERE id = $1', [keyId]);
  res.json({ message: 'API key revoked' });
});
