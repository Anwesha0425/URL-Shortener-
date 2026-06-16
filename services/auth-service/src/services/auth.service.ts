import jwt from 'jsonwebtoken';
import bcrypt from 'bcryptjs';
import { v4 as uuidv4 } from 'uuid';
import { config } from '../config';
import { db } from '../database';
import { redis } from '../cache/redis';
import { logger } from '../utils/logger';

export interface TokenPayload {
  userId: string;
  email:  string;
  tier:   string;
}

export interface AuthTokens {
  accessToken:  string;
  refreshToken: string;
  expiresIn:    number;
}

// ── Registration ──────────────────────────────────────────────────
export async function register(
  email: string,
  password: string,
): Promise<{ userId: string; email: string }> {
  const existing = await db.query(
    'SELECT id FROM users WHERE email = $1',
    [email],
  );
  if (existing.rows.length > 0) {
    throw Object.assign(new Error('Email already registered'), { status: 409 });
  }

  const passwordHash = await bcrypt.hash(password, 12);
  const apiKey       = `uk_${uuidv4().replace(/-/g, '')}`;

  const result = await db.query(
    `INSERT INTO users (email, password_hash, api_key, tier)
     VALUES ($1, $2, $3, 'free')
     RETURNING id, email`,
    [email, passwordHash, apiKey],
  );

  logger.info('User registered', { email });
  return { userId: result.rows[0].id, email: result.rows[0].email };
}

// ── Login ─────────────────────────────────────────────────────────
export async function login(
  email: string,
  password: string,
): Promise<AuthTokens> {
  const result = await db.query(
    'SELECT id, email, password_hash, tier FROM users WHERE email = $1',
    [email],
  );

  const user = result.rows[0];
  if (!user) {
    throw Object.assign(new Error('Invalid credentials'), { status: 401 });
  }

  const valid = await bcrypt.compare(password, user.password_hash);
  if (!valid) {
    throw Object.assign(new Error('Invalid credentials'), { status: 401 });
  }

  return issueTokens({ userId: user.id, email: user.email, tier: user.tier });
}

// ── Refresh ───────────────────────────────────────────────────────
export async function refreshTokens(refreshToken: string): Promise<AuthTokens> {
  // Check if token is blacklisted (logged out)
  const blacklisted = await redis.get(`blacklist:${refreshToken}`);
  if (blacklisted) {
    throw Object.assign(new Error('Token revoked'), { status: 401 });
  }

  let payload: TokenPayload;
  try {
    payload = jwt.verify(refreshToken, config.JWT_SECRET) as TokenPayload;
  } catch {
    throw Object.assign(new Error('Invalid token'), { status: 401 });
  }

  // Blacklist old refresh token (rotation)
  await redis.setex(`blacklist:${refreshToken}`, 7 * 24 * 3600, '1');

  return issueTokens(payload);
}

// ── Logout ────────────────────────────────────────────────────────
export async function logout(refreshToken: string): Promise<void> {
  await redis.setex(`blacklist:${refreshToken}`, 7 * 24 * 3600, '1');
  logger.info('Token blacklisted on logout');
}

// ── Token Verification (used by other services via HTTP) ──────────
export function verifyAccessToken(token: string): TokenPayload {
  try {
    return jwt.verify(token, config.JWT_SECRET) as TokenPayload;
  } catch {
    throw Object.assign(new Error('Invalid or expired token'), { status: 401 });
  }
}

// ── Internal ──────────────────────────────────────────────────────
function issueTokens(payload: TokenPayload): AuthTokens {
  const accessToken = jwt.sign(payload, config.JWT_SECRET, {
    expiresIn: config.JWT_ACCESS_EXPIRES,
    issuer: 'url-shortener',
    audience: 'url-shortener-clients',
  });

  const refreshToken = jwt.sign(payload, config.JWT_SECRET, {
    expiresIn: config.JWT_REFRESH_EXPIRES,
    issuer: 'url-shortener',
    audience: 'url-shortener-clients',
  });

  return { accessToken, refreshToken, expiresIn: 15 * 60 }; // 15 min
}
