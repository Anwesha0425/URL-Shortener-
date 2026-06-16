import { logger } from './utils/logger';
import { sendEmail } from './channels/email';
import { sendWebhook } from './channels/webhook';

/**
 * NotificationDispatcher — routes Kafka events to the correct notification channel.
 *
 * Supported channels:
 *   - Email   (via SMTP / Nodemailer)
 *   - Webhook (via HTTP POST to user-configured URL)
 *
 * Event → Action mapping:
 *   url.created           → Welcome email with short URL details
 *   url.expired           → Expiry alert to URL owner
 *   analytics.milestone   → Click milestone notification (1K, 10K, 100K clicks)
 */
export class NotificationDispatcher {
  async dispatch(topic: string, event: Record<string, unknown>): Promise<void> {
    switch (topic) {
      case 'url.created':
        await this.onURLCreated(event);
        break;

      case 'url.expired':
        await this.onURLExpired(event);
        break;

      case 'analytics.milestone':
        await this.onMilestone(event);
        break;

      default:
        logger.warn('Unknown topic — no handler', { topic });
    }
  }

  private async onURLCreated(event: Record<string, unknown>) {
    const { short_code, original_url, user_email } = event as {
      short_code: string;
      original_url: string;
      user_email: string;
    };

    if (!user_email) return;

    await sendEmail({
      to:      user_email,
      subject: `Your short link is ready: sho.rt/${short_code}`,
      html: `
        <h2>Your short link has been created! 🎉</h2>
        <p><strong>Short URL:</strong> <a href="https://sho.rt/${short_code}">sho.rt/${short_code}</a></p>
        <p><strong>Original URL:</strong> ${original_url}</p>
        <p>You can track clicks and analytics in your dashboard.</p>
      `,
    });
  }

  private async onURLExpired(event: Record<string, unknown>) {
    const { short_code, user_email, webhook_url } = event as {
      short_code:  string;
      user_email?: string;
      webhook_url?: string;
    };

    if (user_email) {
      await sendEmail({
        to:      user_email,
        subject: `Your link sho.rt/${short_code} has expired`,
        html: `
          <h2>Link Expired</h2>
          <p>Your short link <strong>sho.rt/${short_code}</strong> has expired and is no longer active.</p>
          <p>Log in to your dashboard to renew it or create a new link.</p>
        `,
      });
    }

    if (webhook_url) {
      await sendWebhook(webhook_url, { event: 'url.expired', short_code });
    }
  }

  private async onMilestone(event: Record<string, unknown>) {
    const { short_code, milestone, user_email, webhook_url } = event as {
      short_code:   string;
      milestone:    number;
      user_email?:  string;
      webhook_url?: string;
    };

    const formatted = new Intl.NumberFormat('en').format(milestone);
    logger.info(`Milestone reached: ${formatted} clicks on ${short_code}`);

    if (user_email) {
      await sendEmail({
        to:      user_email,
        subject: `🎉 Your link hit ${formatted} clicks!`,
        html: `
          <h2>Milestone Reached! 🚀</h2>
          <p>Your link <strong>sho.rt/${short_code}</strong> just surpassed <strong>${formatted} clicks</strong>!</p>
          <p>Check your analytics dashboard for detailed insights.</p>
        `,
      });
    }

    if (webhook_url) {
      await sendWebhook(webhook_url, { event: 'analytics.milestone', short_code, milestone });
    }
  }
}
