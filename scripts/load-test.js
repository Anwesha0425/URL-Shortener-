/**
 * k6 Load Test — URL Shortener Platform
 *
 * Run:  k6 run scripts/load-test.js
 * With output: k6 run --out influxdb=http://localhost:8086/k6 scripts/load-test.js
 *
 * Test stages simulate real traffic patterns:
 *   1. Ramp up: 0 → 1,000 VUs over 2 min (gradual load)
 *   2. Sustained: 1,000 VUs for 5 min (steady state)
 *   3. Spike: 1,000 → 5,000 VUs over 1 min (traffic spike)
 *   4. Recovery: 5,000 → 500 VUs over 2 min (scale down)
 *   5. Ramp down: 500 → 0 over 1 min (graceful end)
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// ── Custom Metrics ────────────────────────────────────────────────
const redirectLatency = new Trend('redirect_latency_ms', true);
const cacheHitRate    = new Rate('cache_hit_rate');
const errorRate       = new Rate('error_rate');
const totalRedirects  = new Counter('total_redirects');

// ── Test Config ───────────────────────────────────────────────────
export const options = {
  stages: [
    { duration: '2m',  target: 1000  },  // Ramp up
    { duration: '5m',  target: 1000  },  // Sustained load
    { duration: '1m',  target: 5000  },  // Traffic spike
    { duration: '2m',  target: 500   },  // Recovery
    { duration: '1m',  target: 0     },  // Ramp down
  ],
  thresholds: {
    // SLO: p99 redirect latency < 50ms
    redirect_latency_ms:       ['p(99)<50'],
    // SLO: error rate < 0.1%
    error_rate:                ['rate<0.001'],
    // SLO: cache hit rate > 95%
    cache_hit_rate:            ['rate>0.95'],
    // SLO: HTTP error codes < 0.1%
    'http_req_failed':         ['rate<0.001'],
  },
};

const BASE_URL        = __ENV.BASE_URL        || 'http://localhost:8002';
const URL_SERVICE_URL = __ENV.URL_SERVICE_URL || 'http://localhost:8001';

// Pre-seeded short codes (replace with real ones)
const SHORT_CODES = [
  'aB3xY', 'zK9mP', 'Qr7nL', 'Wx2sT', 'Hd5fV',
  'Jc8bN', 'Yp4eM', 'Lr6kS', 'Fg1iO', 'Dn0uA',
];

export function setup() {
  // Create test URLs before load test starts
  const urls = [];
  for (let i = 0; i < 10; i++) {
    const res = http.post(
      `${URL_SERVICE_URL}/api/v1/urls`,
      JSON.stringify({ original_url: `https://example.com/page/${i}` }),
      { headers: { 'Content-Type': 'application/json' } },
    );
    if (res.status === 201) {
      urls.push(JSON.parse(res.body as string).short_code);
    }
  }
  return { shortCodes: urls.length > 0 ? urls : SHORT_CODES };
}

export default function (data: { shortCodes: string[] }) {
  const shortCode = data.shortCodes[Math.floor(Math.random() * data.shortCodes.length)];

  group('redirect', () => {
    const res = http.get(`${BASE_URL}/${shortCode}`, {
      redirects: 0,            // Don't follow redirect — we measure gateway latency
      tags:      { name: 'redirect' },
    });

    const success = check(res, {
      'status is 301 or 302': (r) => r.status === 301 || r.status === 302,
      'has Location header':  (r) => !!r.headers['Location'],
      'latency < 100ms':      (r) => r.timings.duration < 100,
    });

    redirectLatency.add(res.timings.duration);
    totalRedirects.add(1);
    errorRate.add(!success ? 1 : 0);

    // Detect cache hits via custom response header (set by redirect service)
    const cachedHeader = res.headers['X-Cache'];
    cacheHitRate.add(cachedHeader === 'HIT' ? 1 : 0);
  });

  // 5% of traffic creates new URLs (write path)
  if (Math.random() < 0.05) {
    group('create_url', () => {
      const res = http.post(
        `${URL_SERVICE_URL}/api/v1/urls`,
        JSON.stringify({ original_url: `https://example.com/page/${Date.now()}` }),
        { headers: { 'Content-Type': 'application/json' }, tags: { name: 'create_url' } },
      );
      check(res, { 'URL created': (r) => r.status === 201 });
    });
  }

  sleep(0.1); // 100ms think time between requests
}

export function handleSummary(data: Record<string, unknown>) {
  return {
    'load-test-results.json': JSON.stringify(data, null, 2),
    stdout: `
========================================
  Load Test Summary
========================================
  Redirect p99 latency : ${(data as any).metrics?.redirect_latency_ms?.values?.p99?.toFixed(2) || 'N/A'} ms
  Cache hit rate       : ${((data as any).metrics?.cache_hit_rate?.values?.rate * 100)?.toFixed(2) || 'N/A'} %
  Error rate           : ${((data as any).metrics?.error_rate?.values?.rate * 100)?.toFixed(4) || 'N/A'} %
  Total redirects      : ${(data as any).metrics?.total_redirects?.values?.count || 'N/A'}
========================================
`,
  };
}
