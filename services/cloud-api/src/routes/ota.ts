/**
 * OTA (Over-the-Air) update routes
 */

import { Hono } from 'hono';
import type { Env, Device, OTACheckResponse } from '../types';
import { jsonResponse, errorResponse, extractBearerToken, nowISO } from '../utils';

const ota = new Hono<{ Bindings: Env }>();

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
 * Compare semantic versions
 * Returns: -1 if v1 < v2, 0 if equal, 1 if v1 > v2
 */
function compareVersions(v1: string, v2: string): number {
  const parts1 = v1.split('.').map(Number);
  const parts2 = v2.split('.').map(Number);

  for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
    const p1 = parts1[i] || 0;
    const p2 = parts2[i] || 0;
    if (p1 < p2) return -1;
    if (p1 > p2) return 1;
  }
  return 0;
}

/**
 * GET /devices/:deviceId/ota
 * Check for firmware updates.
 */
ota.get('/:deviceId/ota', async (c) => {
  const deviceId = c.req.param('deviceId');
  const device = await verifyDeviceToken(c, deviceId);

  if (!device) {
    return errorResponse('unauthorized', 'Invalid device token', 401);
  }

  const { DB } = c.env;
  const currentVersion = c.req.query('current_version') || device.firmware_version || '0.0.0';

  // Find latest release for this product type
  const latestRelease = await DB.prepare(
    `SELECT version, url, sha256, size_bytes, release_notes, mandatory, min_version
     FROM firmware_releases
     WHERE product_type = ?
     ORDER BY created_at DESC
     LIMIT 1`
  )
    .bind(device.product_type)
    .first<{
      version: string;
      url: string;
      sha256: string;
      size_bytes: number;
      release_notes: string | null;
      mandatory: number;
      min_version: string | null;
    }>();

  if (!latestRelease) {
    const response: OTACheckResponse = {
      update_available: false,
      current_version: currentVersion,
    };
    return jsonResponse(response);
  }

  // Check if update is available
  const updateAvailable = compareVersions(latestRelease.version, currentVersion) > 0;

  // Check min_version requirement
  if (updateAvailable && latestRelease.min_version) {
    if (compareVersions(currentVersion, latestRelease.min_version) < 0) {
      return errorResponse(
        'version_too_old',
        `Current version ${currentVersion} is below minimum required ${latestRelease.min_version}`,
        400
      );
    }
  }

  const response: OTACheckResponse = {
    update_available: updateAvailable,
    current_version: currentVersion,
    ...(updateAvailable && {
      version: latestRelease.version,
      url: latestRelease.url,
      sha256: latestRelease.sha256,
      size_bytes: latestRelease.size_bytes,
      release_notes: latestRelease.release_notes || undefined,
      mandatory: latestRelease.mandatory === 1,
      min_version: latestRelease.min_version || undefined,
    }),
  };

  return jsonResponse(response);
});

/**
 * POST /devices/:deviceId/ota/status
 * Report OTA update status
 */
ota.post('/:deviceId/ota/status', async (c) => {
  const deviceId = c.req.param('deviceId');
  const device = await verifyDeviceToken(c, deviceId);

  if (!device) {
    return errorResponse('unauthorized', 'Invalid device token', 401);
  }

  const body = await c.req.json<{
    version: string;
    status: string;
    from_version?: string;
    error_message?: string;
  }>();

  const { DB } = c.env;

  // Check if there's an existing update record
  const existingUpdate = await DB.prepare(
    `SELECT id FROM device_ota_updates
     WHERE device_id = ? AND to_version = ? AND status != 'completed' AND status != 'failed'
     ORDER BY created_at DESC LIMIT 1`
  )
    .bind(deviceId, body.version)
    .first<{ id: number }>();

  if (existingUpdate) {
    // Update existing record
    await DB.prepare(
      `UPDATE device_ota_updates
       SET status = ?, completed_at = ?, error_message = ?
       WHERE id = ?`
    )
      .bind(
        body.status,
        ['completed', 'failed'].includes(body.status) ? nowISO() : null,
        body.error_message || null,
        existingUpdate.id
      )
      .run();
  } else {
    // Create new record
    await DB.prepare(
      `INSERT INTO device_ota_updates (device_id, from_version, to_version, started_at, status, error_message, created_at)
       VALUES (?, ?, ?, ?, ?, ?, ?)`
    )
      .bind(
        deviceId,
        body.from_version || device.firmware_version || 'unknown',
        body.version,
        nowISO(),
        body.status,
        body.error_message || null,
        nowISO()
      )
      .run();
  }

  // If update completed, update device firmware version
  if (body.status === 'completed') {
    await DB.prepare(
      'UPDATE devices SET firmware_version = ?, updated_at = ? WHERE id = ?'
    )
      .bind(body.version, nowISO(), deviceId)
      .run();
  }

  return jsonResponse({ received: true });
});

export default ota;
