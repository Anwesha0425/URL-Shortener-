import nodemailer from 'nodemailer';
import { logger } from '../utils/logger';

const transporter = nodemailer.createTransport({
  host:   process.env.SMTP_HOST   || 'smtp.gmail.com',
  port:   parseInt(process.env.SMTP_PORT || '587'),
  secure: false,
  auth: {
    user: process.env.SMTP_USER || '',
    pass: process.env.SMTP_PASS || '',
  },
});

interface EmailOptions {
  to:      string;
  subject: string;
  html:    string;
}

export async function sendEmail(opts: EmailOptions): Promise<void> {
  try {
    await transporter.sendMail({
      from:    `"URL Shortener" <${process.env.SMTP_FROM || 'noreply@sho.rt'}>`,
      to:      opts.to,
      subject: opts.subject,
      html:    opts.html,
    });
    logger.info('Email sent', { to: opts.to, subject: opts.subject });
  } catch (err) {
    logger.error('Failed to send email', { to: opts.to, err });
    // Do not re-throw — notification failure should not crash the consumer
  }
}
