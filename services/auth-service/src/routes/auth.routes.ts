import { Router, Request, Response } from 'express';
import { z } from 'zod';
import * as AuthService from '../services/auth.service';
import { validate } from '../middleware/validate';

export const authRouter = Router();

const registerSchema = z.object({
  body: z.object({
    email:    z.string().email(),
    password: z.string().min(8, 'Password must be at least 8 characters'),
  }),
});

const loginSchema = z.object({
  body: z.object({
    email:    z.string().email(),
    password: z.string().min(1),
  }),
});

// POST /api/v1/auth/register
authRouter.post('/register', validate(registerSchema), async (req: Request, res: Response) => {
  const { email, password } = req.body;
  const user = await AuthService.register(email, password);
  res.status(201).json({ message: 'Registration successful', user });
});

// POST /api/v1/auth/login
authRouter.post('/login', validate(loginSchema), async (req: Request, res: Response) => {
  const { email, password } = req.body;
  const tokens = await AuthService.login(email, password);
  res.json(tokens);
});

// POST /api/v1/auth/refresh
authRouter.post('/refresh', async (req: Request, res: Response) => {
  const { refreshToken } = req.body;
  if (!refreshToken) {
    return res.status(400).json({ error: 'refreshToken required' });
  }
  const tokens = await AuthService.refreshTokens(refreshToken);
  res.json(tokens);
});

// POST /api/v1/auth/logout
authRouter.post('/logout', async (req: Request, res: Response) => {
  const { refreshToken } = req.body;
  if (refreshToken) await AuthService.logout(refreshToken);
  res.json({ message: 'Logged out successfully' });
});

// GET /api/v1/auth/verify  — used by API Gateway to validate tokens
authRouter.get('/verify', async (req: Request, res: Response) => {
  const authHeader = req.headers.authorization;
  if (!authHeader?.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'No token provided' });
  }
  const token = authHeader.slice(7);
  const payload = AuthService.verifyAccessToken(token);
  res.json({ valid: true, payload });
});
