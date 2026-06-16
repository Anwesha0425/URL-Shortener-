/**
 * Cloudflare Worker — Edge URL Redirect
 *
 * Deployed globally to 300+ Cloudflare PoPs.
 * Resolves short URLs at the edge for < 1ms latency worldwide.
 *
 * Resolution chain:
 *   1. Cloudflare KV (edge cache) — fastest, ~0ms
 *   2. Origin redirect-service    — fallback, ~5-50ms
 *
 * Cache strategy:
 *   - KV TTL: 1 hour (configurable per URL)
 *   - Cache-Control: public, max-age=300, stale-while-revalidate=60
 *
 * Security:
 *   - Rate limiting via Cloudflare Rate Limit rules (configured in dashboard)
 *   - DDoS protection is automatic via Cloudflare
 *   - Bot fingerprinting via cf-bot-management header
 */

export interface Env {
  URL_CACHE:       KVNamespace;   // Cloudflare KV for edge caching
  ORIGIN_URL:      string;        // e.g. https://api.sho.rt
  CACHE_TTL:       string;        // seconds (default: 3600)
  ANALYTICS_TOPIC: string;        // Kafka topic name (unused at edge — sent to origin)
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url  = new URL(request.url);
    const path = url.pathname;

    // ── Skip non-short-code paths ──────────────────────────────────
    if (path === '/' || path === '/health' || path.startsWith('/api')) {
      return fetch(request);
    }

    // ── Extract short code ─────────────────────────────────────────
    // e.g. /aB3xY or /aB3xY/extra-path → short code is "aB3xY"
    const shortCode = path.split('/')[1];
    if (!shortCode || shortCode.length > 20) {
      return new Response('Not Found', { status: 404 });
    }

    // ── Step 1: Check KV edge cache ───────────────────────────────
    const cached = await env.URL_CACHE.get(shortCode);
    if (cached) {
      return buildRedirect(cached, shortCode, 'HIT');
    }

    // ── Step 2: Fetch from origin ─────────────────────────────────
    const originResp = await fetch(
      `${env.ORIGIN_URL}/${shortCode}`,
      {
        redirect: 'manual',        // Don't follow redirect
        headers: {
          'X-Forwarded-For': request.headers.get('CF-Connecting-IP') || '',
          'X-Country':       request.cf?.country || '',
          'X-CF-Ray':        request.headers.get('CF-Ray') || '',
        },
      }
    );

    // Handle non-redirect responses
    if (originResp.status !== 301 && originResp.status !== 302) {
      return originResp;
    }

    const originalURL = originResp.headers.get('Location');
    if (!originalURL) {
      return new Response('Bad Gateway', { status: 502 });
    }

    // ── Step 3: Store in KV (async — don't block response) ────────
    const ttl = parseInt(env.CACHE_TTL || '3600');
    ctx.waitUntil(
      env.URL_CACHE.put(shortCode, originalURL, { expirationTtl: ttl })
    );

    return buildRedirect(originalURL, shortCode, 'MISS');
  },
};

function buildRedirect(destination: string, shortCode: string, cacheStatus: 'HIT' | 'MISS'): Response {
  return new Response(null, {
    status: 302,
    headers: {
      'Location':       destination,
      'Cache-Control':  'public, max-age=300, stale-while-revalidate=60',
      'X-Cache':        cacheStatus,
      'X-Short-Code':   shortCode,
      'Vary':           'Accept-Encoding',
    },
  });
}
