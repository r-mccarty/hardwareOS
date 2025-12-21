/**
 * Telemetry routes for device metrics and events
 */

import { Hono } from 'hono';
import type { Env, Device, TelemetryRequest } from '../types';
import { generateId, nowISO, jsonResponse, errorResponse, extractBearerToken } from '../utils';

const telemetry = new Hono<{ Bindings: Env }>();

/**
 * Middleware to verify device token
 */
async function verifyDeviceToken(
  c: { req: { header: (name: string) => string | undefined }; env: Env },
  deviceId: string
): Promise<Device | null> {
  const token = extractBearerToken(c.req.header('Authorization') || null);
  if (!token) return null;

  const { DB } = c.env;
  return DB.prepare(
    'SELECT id, serial_number, product_type, firmware_version, customer_id FROM devices WHERE id = ? AND cloud_token = ?'
  )
    .bind(deviceId, token)
    .first<Device>();
}

/**
 * POST /devices/:deviceId/telemetry
 * Submit device telemetry data.
 */
telemetry.post('/:deviceId/telemetry', async (c) => {
  const deviceId = c.req.param('deviceId');
  const device = await verifyDeviceToken(c, deviceId);

  if (!device) {
    return errorResponse('unauthorized', 'Invalid device token', 401);
  }

  const body = await c.req.json<TelemetryRequest>();

  if (!body.timestamp || !body.metrics) {
    return errorResponse('invalid_request', 'timestamp and metrics are required', 400);
  }

  const { DB } = c.env;

  // Insert telemetry record
  await DB.prepare(
    `INSERT INTO device_telemetry (device_id, timestamp, metrics, events, created_at)
     VALUES (?, ?, ?, ?, ?)`
  )
    .bind(
      deviceId,
      body.timestamp,
      JSON.stringify(body.metrics),
      body.events ? JSON.stringify(body.events) : null,
      nowISO()
    )
    .run();

  // Update device last_seen_at
  await DB.prepare('UPDATE devices SET last_seen_at = ?, updated_at = ? WHERE id = ?')
    .bind(nowISO(), nowISO(), deviceId)
    .run();

  return jsonResponse({ received: true });
});

/**
 * GET /devices/:deviceId/telemetry
 * Get recent telemetry for a device (admin/debug endpoint)
 */
telemetry.get('/:deviceId/telemetry', async (c) => {
  const deviceId = c.req.param('deviceId');
  const device = await verifyDeviceToken(c, deviceId);

  if (!device) {
    return errorResponse('unauthorized', 'Invalid device token', 401);
  }

  const { DB } = c.env;
  const limit = parseInt(c.req.query('limit') || '100');

  const telemetryData = await DB.prepare(
    `SELECT id, timestamp, metrics, events, created_at
     FROM device_telemetry
     WHERE device_id = ?
     ORDER BY timestamp DESC
     LIMIT ?`
  )
    .bind(deviceId, Math.min(limit, 1000))
    .all<{ id: number; timestamp: string; metrics: string; events: string | null; created_at: string }>();

  const results = (telemetryData.results || []).map((row) => ({
    id: row.id,
    timestamp: row.timestamp,
    metrics: JSON.parse(row.metrics),
    events: row.events ? JSON.parse(row.events) : null,
    created_at: row.created_at,
  }));

  return jsonResponse({ telemetry: results });
});

export default telemetry;
