/**
 * Device management routes
 * Based on docs/CLOUD_API.md specification
 */

import { Hono } from 'hono';
import type { Env, Device, RegisterDeviceRequest, TokenExchangeRequest } from '../types';
import { generateToken, generateId, nowISO, jsonResponse, errorResponse, parseCookies, hashToken } from '../utils';

const devices = new Hono<{ Bindings: Env }>();

const SESSION_COOKIE_NAME = 'opticworks_session';

/**
 * GET /devices
 * List all devices for the authenticated user (used by frontend)
 */
devices.get('/', async (c) => {
  const { DB } = c.env;
  const cookies = parseCookies(c.req.header('Cookie'));
  const sessionToken = cookies[SESSION_COOKIE_NAME];

  if (!sessionToken) {
    return errorResponse('unauthorized', 'Not authenticated', 401);
  }

  const tokenHash = await hashToken(sessionToken);

  // Find valid session
  const session = await DB.prepare(
    `SELECT s.user_id
     FROM user_sessions s
     WHERE s.token_hash = ? AND s.expires_at > datetime('now')`
  )
    .bind(tokenHash)
    .first<{ user_id: string }>();

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

  // Transform to frontend expected format
  const devices = (devicesList.results || []).map((d) => ({
    id: d.id,
    name: d.serial_number, // Use serial as display name
    online: d.last_seen_at ? (Date.now() - new Date(d.last_seen_at).getTime()) < 5 * 60 * 1000 : false,
    lastSeen: d.last_seen_at || '',
    version: d.firmware_version || 'unknown',
  }));

  return jsonResponse({ devices });
});

/**
 * POST /devices/register
 * Device registration at first boot. Returns a temporary token.
 */
devices.post('/register', async (c) => {
  const body = await c.req.json<RegisterDeviceRequest>();

  if (!body.serial_number) {
    return errorResponse('invalid_request', 'serial_number is required', 400);
  }

  const { DB } = c.env;
  const serialNumber = body.serial_number;
  const productType = body.product_type || 'rs1';
  const firmwareVersion = body.firmware_version || null;

  // Check if device already exists
  const existing = await DB.prepare(
    'SELECT id, cloud_token, customer_id FROM devices WHERE serial_number = ?'
  )
    .bind(serialNumber)
    .first<Pick<Device, 'id' | 'cloud_token' | 'customer_id'>>();

  if (existing) {
    // Device already registered
    if (existing.cloud_token && existing.customer_id) {
      return errorResponse(
        'already_registered',
        'Device is already registered',
        409
      );
    }
    // Device exists but not fully paired - return existing temp token or generate new one
    const tempToken = generateToken(32);
    await DB.prepare(
      'UPDATE devices SET temp_token = ?, firmware_version = ?, updated_at = ? WHERE id = ?'
    )
      .bind(tempToken, firmwareVersion, nowISO(), existing.id)
      .run();

    return jsonResponse({
      temp_token: tempToken,
      device_id: existing.id,
      status: 'awaiting_pairing' as const,
    });
  }

  // Create new device
  const deviceId = generateId('dev');
  const tempToken = generateToken(32);

  await DB.prepare(
    `INSERT INTO devices (id, serial_number, product_type, firmware_version, temp_token, registered_at, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
  )
    .bind(deviceId, serialNumber, productType, firmwareVersion, tempToken, nowISO(), nowISO(), nowISO())
    .run();

  return jsonResponse({
    temp_token: tempToken,
    device_id: deviceId,
    status: 'awaiting_pairing' as const,
  });
});

/**
 * POST /devices/token
 * Exchange temporary token for permanent cloud token after customer pairing.
 */
devices.post('/token', async (c) => {
  const body = await c.req.json<TokenExchangeRequest>();

  if (!body.temp_token) {
    return errorResponse('invalid_request', 'temp_token is required', 400);
  }

  const { DB } = c.env;

  // Find device by temp token
  const device = await DB.prepare(
    'SELECT id, serial_number, customer_id FROM devices WHERE temp_token = ?'
  )
    .bind(body.temp_token)
    .first<Pick<Device, 'id' | 'serial_number' | 'customer_id'>>();

  if (!device) {
    return errorResponse('invalid_token', 'Token is invalid or expired', 401);
  }

  if (!device.customer_id) {
    return errorResponse('not_paired', 'Device not yet paired to a customer', 412);
  }

  // Generate permanent cloud token
  const cloudToken = generateToken(64);

  await DB.prepare(
    'UPDATE devices SET cloud_token = ?, temp_token = NULL, paired_at = ?, updated_at = ? WHERE id = ?'
  )
    .bind(cloudToken, nowISO(), nowISO(), device.id)
    .run();

  return jsonResponse({
    cloud_token: cloudToken,
    device_id: device.id,
    customer_id: device.customer_id,
  });
});

/**
 * DELETE /devices/:deviceId
 * Deregister device from cloud.
 */
devices.delete('/:deviceId', async (c) => {
  const deviceId = c.req.param('deviceId');
  const authHeader = c.req.header('Authorization');
  const token = authHeader?.replace(/^Bearer\s+/i, '');

  if (!token) {
    return errorResponse('unauthorized', 'Missing or invalid authorization', 401);
  }

  const { DB } = c.env;

  // Verify token belongs to this device
  const device = await DB.prepare(
    'SELECT id FROM devices WHERE id = ? AND cloud_token = ?'
  )
    .bind(deviceId, token)
    .first<Pick<Device, 'id'>>();

  if (!device) {
    return errorResponse('unauthorized', 'Invalid token for this device', 401);
  }

  // Clear cloud token and customer association
  await DB.prepare(
    'UPDATE devices SET cloud_token = NULL, customer_id = NULL, paired_at = NULL, updated_at = ? WHERE id = ?'
  )
    .bind(nowISO(), deviceId)
    .run();

  return jsonResponse({ success: true });
});

/**
 * GET /devices/:deviceId
 * Get device info (used by cloud UI)
 */
devices.get('/:deviceId', async (c) => {
  const deviceId = c.req.param('deviceId');
  const authHeader = c.req.header('Authorization');
  const token = authHeader?.replace(/^Bearer\s+/i, '');

  if (!token) {
    return errorResponse('unauthorized', 'Missing or invalid authorization', 401);
  }

  const { DB } = c.env;

  // Get device - verify token matches
  const device = await DB.prepare(
    `SELECT id, serial_number, product_type, firmware_version, customer_id,
            last_seen_at, registered_at, paired_at, metadata
     FROM devices
     WHERE id = ? AND cloud_token = ?`
  )
    .bind(deviceId, token)
    .first<Device>();

  if (!device) {
    return errorResponse('device_not_found', 'Device not found', 404);
  }

  return jsonResponse({
    id: device.id,
    serial_number: device.serial_number,
    product_type: device.product_type,
    firmware_version: device.firmware_version,
    customer_id: device.customer_id,
    last_seen_at: device.last_seen_at,
    registered_at: device.registered_at,
    paired_at: device.paired_at,
  });
});

/**
 * POST /devices/:deviceId/pair
 * Pair a device to a user (called from the UI after user scans QR or enters code)
 */
devices.post('/:deviceId/pair', async (c) => {
  const deviceId = c.req.param('deviceId');
  const body = await c.req.json<{ user_id: string; temp_token?: string }>();

  if (!body.user_id) {
    return errorResponse('invalid_request', 'user_id is required', 400);
  }

  const { DB } = c.env;

  // Find device
  const device = await DB.prepare('SELECT id, customer_id FROM devices WHERE id = ?')
    .bind(deviceId)
    .first<Pick<Device, 'id' | 'customer_id'>>();

  if (!device) {
    return errorResponse('device_not_found', 'Device not found', 404);
  }

  if (device.customer_id) {
    return errorResponse('already_paired', 'Device is already paired to a customer', 409);
  }

  // Pair device to user
  await DB.prepare(
    'UPDATE devices SET customer_id = ?, paired_at = ?, updated_at = ? WHERE id = ?'
  )
    .bind(body.user_id, nowISO(), nowISO(), deviceId)
    .run();

  return jsonResponse({ success: true, device_id: deviceId, customer_id: body.user_id });
});

export default devices;
