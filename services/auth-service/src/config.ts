import dotenv from 'dotenv';
dotenv.config();

export const config = {
  PORT: process.env.PORT || '8004',

  // Database
  DB_HOST:     process.env.DB_HOST     || 'localhost',
  DB_PORT:     parseInt(process.env.DB_PORT || '5432'),
  DB_USER:     process.env.DB_USER     || 'urluser',
  DB_PASSWORD: process.env.DB_PASSWORD || 'urlpassword',
  DB_NAME:     process.env.DB_NAME     || 'urldb',

  // Redis
  REDIS_HOST: process.env.REDIS_HOST || 'localhost',
  REDIS_PORT: parseInt(process.env.REDIS_PORT || '6379'),

  // JWT
  JWT_SECRET:         process.env.JWT_SECRET          || 'change-me-in-production-use-256bit-secret',
  JWT_ACCESS_EXPIRES:  process.env.JWT_ACCESS_EXPIRES  || '15m',
  JWT_REFRESH_EXPIRES: process.env.JWT_REFRESH_EXPIRES || '7d',

  // Rate limiting (requests per window)
  RATE_LIMIT_FREE_RPS:       parseInt(process.env.RATE_LIMIT_FREE_RPS       || '10'),
  RATE_LIMIT_PRO_RPS:        parseInt(process.env.RATE_LIMIT_PRO_RPS        || '100'),
  RATE_LIMIT_ENTERPRISE_RPS: parseInt(process.env.RATE_LIMIT_ENTERPRISE_RPS || '1000'),
};
