import axios from 'axios';
import { logger } from '../utils/logger';

export async function sendWebhook(
  url:     string,
  payload: Record<string, unknown>,
  retries  = 3,
): Promise<void> {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      await axios.post(url, payload, {
        timeout: 5000,
        headers: { 'Content-Type': 'application/json', 'X-Source': 'url-shortener' },
      });
      logger.info('Webhook delivered', { url, attempt });
      return;
    } catch (err) {
      logger.warn(`Webhook attempt ${attempt}/${retries} failed`, { url, err });
      if (attempt < retries) {
        // Exponential backoff: 1s, 2s, 4s
        await new Promise((r) => setTimeout(r, 1000 * 2 ** (attempt - 1)));
      }
    }
  }
  logger.error('Webhook delivery failed after all retries', { url });
}
