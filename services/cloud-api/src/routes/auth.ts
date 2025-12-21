/**
 * Authentication routes for cloud frontend
 * Handles user login/logout and session management
 */

import { Hono } from 'hono';
import type { Env, User, Session, MeResponse } from '../types';
import {
  generateToken,
  generateId,
  hashToken,
  nowISO,
  jsonResponse,
  errorResponse,
  parseCookies,
  setCookie,
} from '../utils';

const auth = new Hono<{ Bindings: Env }>();

const SESSION_COOKIE_NAME = 'opticworks_session';
const SESSION_MAX_AGE = 60 * 60 * 24 * 7; // 7 days

/**
 * GET /me
 * Get current user info. Returns 401 if not authenticated.
 */
auth.get('/me', async (c) => {
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
  const devices = await DB.prepare(
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
    devices: devices.results || [],
  };

  return jsonResponse(response);
});

/**
 * POST /auth/login
 * Login with email (simplified - no password for development)
 * In production, this would verify credentials or use OAuth
 */
auth.post('/login', async (c) => {
  const body = await c.req.json<{ email: string; password?: string }>();

  if (!body.email) {
    return errorResponse('invalid_request', 'email is required', 400);
  }

  const { DB } = c.env;

  // Find or create user (simplified for development)
  let user = await DB.prepare('SELECT id, email, name FROM users WHERE email = ?')
    .bind(body.email)
    .first<Pick<User, 'id' | 'email' | 'name'>>();

  if (!user) {
    // Create new user (development mode - in production would require proper auth)
    const userId = generateId('usr');
    await DB.prepare(
      'INSERT INTO users (id, email, created_at, updated_at) VALUES (?, ?, ?, ?)'
    )
      .bind(userId, body.email, nowISO(), nowISO())
      .run();
    user = { id: userId, email: body.email, name: null };
  }

  // Create session
  const sessionToken = generateToken(32);
  const tokenHash = await hashToken(sessionToken);
  const sessionId = generateId('sess');
  const expiresAt = new Date(Date.now() + SESSION_MAX_AGE * 1000).toISOString();

  await DB.prepare(
    'INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)'
  )
    .bind(sessionId, user.id, tokenHash, expiresAt, nowISO())
    .run();

  // Return response with session cookie
  const response = jsonResponse({
    success: true,
    user: { id: user.id, email: user.email, name: user.name },
  });

  response.headers.set(
    'Set-Cookie',
    setCookie(SESSION_COOKIE_NAME, sessionToken, {
      maxAge: SESSION_MAX_AGE,
      path: '/',
      httpOnly: true,
      sameSite: 'Lax',
      secure: c.env.ENVIRONMENT === 'production',
    })
  );

  return response;
});

/**
 * POST /auth/logout
 * Logout and clear session
 */
auth.post('/logout', async (c) => {
  const { DB } = c.env;
  const cookies = parseCookies(c.req.header('Cookie'));
  const sessionToken = cookies[SESSION_COOKIE_NAME];

  if (sessionToken) {
    const tokenHash = await hashToken(sessionToken);
    await DB.prepare('DELETE FROM user_sessions WHERE token_hash = ?')
      .bind(tokenHash)
      .run();
  }

  const response = jsonResponse({ success: true });
  response.headers.set(
    'Set-Cookie',
    setCookie(SESSION_COOKIE_NAME, '', {
      maxAge: 0,
      path: '/',
    })
  );

  return response;
});

/**
 * POST /auth/signup
 * Create a new user account
 */
auth.post('/signup', async (c) => {
  const body = await c.req.json<{ email: string; name?: string }>();

  if (!body.email) {
    return errorResponse('invalid_request', 'email is required', 400);
  }

  const { DB } = c.env;

  // Check if user exists
  const existing = await DB.prepare('SELECT id FROM users WHERE email = ?')
    .bind(body.email)
    .first<Pick<User, 'id'>>();

  if (existing) {
    return errorResponse('user_exists', 'User with this email already exists', 409);
  }

  // Create user
  const userId = generateId('usr');
  await DB.prepare(
    'INSERT INTO users (id, email, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)'
  )
    .bind(userId, body.email, body.name || null, nowISO(), nowISO())
    .run();

  // Create session
  const sessionToken = generateToken(32);
  const tokenHash = await hashToken(sessionToken);
  const sessionId = generateId('sess');
  const expiresAt = new Date(Date.now() + SESSION_MAX_AGE * 1000).toISOString();

  await DB.prepare(
    'INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)'
  )
    .bind(sessionId, userId, tokenHash, expiresAt, nowISO())
    .run();

  const response = jsonResponse({
    success: true,
    user: { id: userId, email: body.email, name: body.name || null },
  });

  response.headers.set(
    'Set-Cookie',
    setCookie(SESSION_COOKIE_NAME, sessionToken, {
      maxAge: SESSION_MAX_AGE,
      path: '/',
      httpOnly: true,
      sameSite: 'Lax',
      secure: c.env.ENVIRONMENT === 'production',
    })
  );

  return response;
});

export default auth;
