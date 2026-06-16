import { redis } from './redis';
import { config } from '../config';
import { logger } from '../utils/logger';

/**
 * Token Bucket Rate Limiter — implemented in Redis using atomic Lua script.
 *
 * Algorithm: Token Bucket
 *   - Each user/API key has a bucket with a max capacity (burst)
 *   - Tokens refill at a steady rate (RPS)
 *   - Each request consumes 1 token
 *   - If bucket is empty → 429 Too Many Requests
 *
 * Why Token Bucket over Fixed Window?
 *   - Allows short bursts (burst capacity > RPS)
 *   - No thundering herd at window boundaries
 *   - Smooth, continuous rate control
 *
 * Implementation: Lua script runs atomically in Redis — no race conditions.
 */

// Lua script: atomically check and consume a token
const TOKEN_BUCKET_SCRIPT = `
local key        = KEYS[1]
local capacity   = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now        = tonumber(ARGV[3])
local cost       = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens      = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now

-- Calculate tokens to add based on elapsed time
local elapsed = math.max(0, now - last_refill)
local refilled = math.floor(elapsed * refill_rate / 1000)
tokens = math.min(capacity, tokens + refilled)

if tokens < cost then
  -- Not enough tokens — rate limited
  redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
  redis.call('EXPIRE', key, 3600)
  return {0, tokens, capacity}
end

tokens = tokens - cost
redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
redis.call('EXPIRE', key, 3600)
return {1, tokens, capacity}
`;

export type Tier = 'free' | 'pro' | 'enterprise';

interface RateLimitResult {
  allowed: boolean;
  remaining: number;
  limit: number;
  retryAfterMs?: number;
}

const TIER_CONFIG: Record<Tier, { rps: number; burst: number }> = {
  free:       { rps: config.RATE_LIMIT_FREE_RPS,       burst: config.RATE_LIMIT_FREE_RPS       * 5  },
  pro:        { rps: config.RATE_LIMIT_PRO_RPS,        burst: config.RATE_LIMIT_PRO_RPS        * 5  },
  enterprise: { rps: config.RATE_LIMIT_ENTERPRISE_RPS, burst: config.RATE_LIMIT_ENTERPRISE_RPS * 5  },
};

export async function checkRateLimit(
  identifier: string,  // user ID or API key
  tier: Tier = 'free',
): Promise<RateLimitResult> {
  const { rps, burst } = TIER_CONFIG[tier];
  const key  = `rate_limit:${tier}:${identifier}`;
  const now  = Date.now();

  try {
    const result = await redis.eval(
      TOKEN_BUCKET_SCRIPT,
      1,       // number of keys
      key,     // KEYS[1]
      burst,   // ARGV[1] — max tokens (burst capacity)
      rps,     // ARGV[2] — refill rate tokens/sec
      now,     // ARGV[3] — current timestamp ms
      1,       // ARGV[4] — tokens to consume
    ) as [number, number, number];

    const [allowed, remaining, limit] = result;

    if (!allowed) {
      const msPerToken = 1000 / rps;
      return { allowed: false, remaining: 0, limit, retryAfterMs: msPerToken };
    }

    return { allowed: true, remaining, limit };
  } catch (err) {
    // If Redis is down, fail open (allow request) to avoid blocking users
    logger.warn('Rate limiter Redis error — failing open', { err });
    return { allowed: true, remaining: -1, limit: burst };
  }
}
