/**
 * OpticWorks Cloud API
 *
 * A Cloudflare Worker-based API for device management, authentication,
 * OTA updates, and telemetry.
 *
 * Based on docs/CLOUD_API.md specification.
 */

import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { logger } from 'hono/logger';
import type { Env, MeResponse } from './types';
import { jsonResponse, errorResponse, parseCookies, hashToken } from './utils';

// Import routes
import devices from './routes/devices';
import auth from './routes/auth';
import telemetry from './routes/telemetry';
import ota from './routes/ota';

const app = new Hono<{ Bindings: Env }>();

const SESSION_COOKIE_NAME = 'opticworks_session';

// Global middleware
app.use('*', logger());
app.use(
  '*',
  cors({
    origin: (origin) => {
      // Allow localhost for development
      if (!origin) return '*';
      if (origin.includes('localhost')) return origin;
      if (origin.includes('127.0.0.1')) return origin;
      // Allow Cloudflare Pages deployments
      if (origin.includes('pages.dev')) return origin;
      // Allow production domains
      if (origin.includes('optic.works')) return origin;
      if (origin.includes('jetkvm.com')) return origin;
      return origin;
    },
    allowMethods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
    allowHeaders: ['Content-Type', 'Authorization', 'X-Device-ID', 'X-App-Version'],
    credentials: true,
    maxAge: 86400,
  })
);

// Health check
app.get('/', (c) => {
  return jsonResponse({
    name: 'OpticWorks Cloud API',
    version: '1.0.0',
    status: 'healthy',
    environment: c.env.ENVIRONMENT,
  });
});

app.get('/health', (c) => {
  return jsonResponse({ status: 'ok' });
});

// Mount routes
app.route('/devices', devices);
app.route('/devices', telemetry);
app.route('/devices', ota);
app.route('/auth', auth);

// /me endpoint at root level (frontend expects this at /me not /auth/me)
app.get('/me', async (c) => {
  const { DB } = c.env;
  const cookies = parseCookies(c.req.header('Cookie'));
  const sessionToken = cookies[SESSION_COOKIE_NAME];

  if (!sessionToken) {
    return errorResponse('unauthorized', 'Not authenticated', 401);
  }

  const tokenHash = await hashToken(sessionToken);

  // Find valid session
  const session = await DB.prepare(
    `SELECT s.id, s.user_id, s.expires_at, u.email, u.name
     FROM user_sessions s
     JOIN users u ON u.id = s.user_id
     WHERE s.token_hash = ? AND s.expires_at > datetime('now')`
  )
    .bind(tokenHash)
    .first<{ id: string; user_id: string; expires_at: string; email: string; name: string | null }>();

  if (!session) {
    return errorResponse('unauthorized', 'Session expired or invalid', 401);
  }

  // Get user's devices
  const devicesList = await DB.prepare(
    `SELECT id, serial_number, product_type, firmware_version, last_seen_at
     FROM devices
     WHERE customer_id = ?`
  )
    .bind(session.user_id)
    .all<{
      id: string;
      serial_number: string;
      product_type: string;
      firmware_version: string | null;
      last_seen_at: string | null;
    }>();

  const response: MeResponse = {
    id: session.user_id,
    email: session.email,
    name: session.name,
    devices: devicesList.results || [],
  };

  return jsonResponse(response);
});

// Device status endpoint (for frontend compatibility)
app.get('/device/status', (c) => {
  // This is called by the frontend when running on-device
  // In cloud mode, we redirect to /me
  return jsonResponse({
    isSetup: true,
    isOnDevice: false,
  });
});

// Device endpoint (for frontend compatibility)
app.get('/device', async (c) => {
  // Return device info if authenticated, 401 otherwise
  // This is the on-device endpoint; in cloud mode, use /me
  return errorResponse('not_on_device', 'This endpoint is for on-device access only', 404);
});

// 404 handler
app.notFound((c) => {
  return errorResponse('not_found', `Route ${c.req.method} ${c.req.path} not found`, 404);
});

// Error handler
app.onError((err, c) => {
  console.error('Unhandled error:', err);
  return errorResponse('internal_error', err.message || 'Internal server error', 500);
});

export default app;
