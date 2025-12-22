/**
 * @file isp.c
 * @brief Rockchip ISP Implementation for RS-1 Vision Pipeline
 *
 * Implements ISP control using the Rockchip AIQ (Auto Image Quality) API.
 * Provides 3A processing (AE/AWB/AF) and LDCH lens distortion correction.
 *
 * Note: This is a spike implementation. Full implementation requires
 * the rkaiq library headers which are part of the Rockchip SDK.
 */

#define _POSIX_C_SOURCE 200809L

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <pthread.h>
#include <math.h>

#include "isp.h"
#include "log.h"

// Forward declarations for RKAIQ types (spike - actual headers from SDK)
// In production, include: <rkaiq/rk_aiq_user_api2_sysctl.h>
typedef void* rk_aiq_sys_ctx_t;

// Module state
static struct {
    rk_aiq_sys_ctx_t*   aiq_ctx;
    isp_config_t        config;
    isp_status_t        status;
    char                sensor_name[64];
    pthread_mutex_t     mutex;
    bool                aiq_available;
} g_isp = {
    .aiq_ctx = NULL,
    .mutex = PTHREAD_MUTEX_INITIALIZER,
    .aiq_available = false,
};

// Stub implementations for RKAIQ API (spike mode)
// These would be replaced with actual SDK calls in production

static int rkaiq_init_stub(const char* sensor_name, const char* iq_path)
{
    log_info("[ISP STUB] Initializing AIQ for sensor: %s, IQ path: %s",
             sensor_name, iq_path);
    // In production: rk_aiq_uapi2_sysctl_init()
    return 0;
}

static int rkaiq_start_stub(void)
{
    log_info("[ISP STUB] Starting AIQ processing");
    // In production: rk_aiq_uapi2_sysctl_start()
    return 0;
}

static int rkaiq_stop_stub(void)
{
    log_info("[ISP STUB] Stopping AIQ processing");
    // In production: rk_aiq_uapi2_sysctl_stop()
    return 0;
}

static void rkaiq_deinit_stub(void)
{
    log_info("[ISP STUB] Deinitializing AIQ");
    // In production: rk_aiq_uapi2_sysctl_deinit()
}

static int rkaiq_set_ldch_enable_stub(bool enable)
{
    log_info("[ISP STUB] LDCH enable: %d", enable);
    // In production: rk_aiq_uapi2_setLdchEn()
    return 0;
}

static int rkaiq_set_ldch_level_stub(uint8_t level)
{
    log_info("[ISP STUB] LDCH level: %d", level);
    // In production: rk_aiq_uapi2_setLdchCorrectLevel()
    return 0;
}

static int rkaiq_set_ldch_mesh_stub(const char* mesh_path)
{
    log_info("[ISP STUB] LDCH mesh path: %s", mesh_path);
    // In production: rk_aiq_uapi2_setLdchMeshPath()
    return 0;
}

int isp_init(const char* sensor_name, const isp_config_t* config)
{
    int ret;

    pthread_mutex_lock(&g_isp.mutex);

    if (g_isp.status.initialized) {
        log_warn("ISP already initialized");
        pthread_mutex_unlock(&g_isp.mutex);
        return 0;
    }

    // Store sensor name
    if (sensor_name) {
        strncpy(g_isp.sensor_name, sensor_name, sizeof(g_isp.sensor_name) - 1);
        g_isp.sensor_name[sizeof(g_isp.sensor_name) - 1] = '\0';
    } else {
        strcpy(g_isp.sensor_name, "sc3336");  // Default sensor
    }

    // Apply configuration
    if (config) {
        g_isp.config = *config;
    } else {
        // Default configuration
        g_isp.config.ldch_enabled = true;
        g_isp.config.ldch_level = ISP_LDCH_LEVEL_FULL;
        strcpy(g_isp.config.ldch_mesh_path, ISP_LDCH_MESH_PATH);
        g_isp.config.hdr_enabled = false;
        g_isp.config.noise_reduction = true;
        g_isp.config.auto_exposure = true;
        g_isp.config.auto_white_balance = true;
    }

    log_info("Initializing ISP for sensor: %s", g_isp.sensor_name);
    log_info("  LDCH enabled: %d, level: %d",
             g_isp.config.ldch_enabled, g_isp.config.ldch_level);

    // Initialize RKAIQ
    ret = rkaiq_init_stub(g_isp.sensor_name, ISP_IQ_FILES_PATH);
    if (ret < 0) {
        log_error("Failed to initialize AIQ: %d", ret);
        pthread_mutex_unlock(&g_isp.mutex);
        return ret;
    }

    // Mark AIQ as available (in stub mode, always succeeds)
    g_isp.aiq_available = true;

    // Apply LDCH configuration
    if (g_isp.config.ldch_enabled) {
        ret = rkaiq_set_ldch_enable_stub(true);
        if (ret < 0) {
            log_warn("Failed to enable LDCH: %d", ret);
        }

        ret = rkaiq_set_ldch_level_stub(g_isp.config.ldch_level);
        if (ret < 0) {
            log_warn("Failed to set LDCH level: %d", ret);
        }

        // Load custom mesh if available
        if (strlen(g_isp.config.ldch_mesh_path) > 0) {
            ret = rkaiq_set_ldch_mesh_stub(g_isp.config.ldch_mesh_path);
            if (ret < 0) {
                log_warn("Failed to load LDCH mesh (using default): %d", ret);
            }
        }
    }

    // Update status
    g_isp.status.initialized = true;
    g_isp.status.running = false;
    g_isp.status.ldch_enabled = g_isp.config.ldch_enabled;
    g_isp.status.ldch_level = g_isp.config.ldch_level;

    pthread_mutex_unlock(&g_isp.mutex);

    log_info("ISP initialized successfully");
    return 0;
}

int isp_start(void)
{
    int ret;

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        log_error("ISP not initialized");
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    if (g_isp.status.running) {
        log_warn("ISP already running");
        pthread_mutex_unlock(&g_isp.mutex);
        return 0;
    }

    ret = rkaiq_start_stub();
    if (ret < 0) {
        log_error("Failed to start AIQ: %d", ret);
        pthread_mutex_unlock(&g_isp.mutex);
        return ret;
    }

    g_isp.status.running = true;

    pthread_mutex_unlock(&g_isp.mutex);

    log_info("ISP processing started");
    return 0;
}

int isp_stop(void)
{
    int ret;

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.running) {
        pthread_mutex_unlock(&g_isp.mutex);
        return 0;
    }

    ret = rkaiq_stop_stub();
    if (ret < 0) {
        log_warn("Failed to stop AIQ: %d", ret);
    }

    g_isp.status.running = false;

    pthread_mutex_unlock(&g_isp.mutex);

    log_info("ISP processing stopped");
    return 0;
}

void isp_shutdown(void)
{
    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return;
    }

    if (g_isp.status.running) {
        pthread_mutex_unlock(&g_isp.mutex);
        isp_stop();
        pthread_mutex_lock(&g_isp.mutex);
    }

    rkaiq_deinit_stub();

    g_isp.aiq_available = false;
    g_isp.status.initialized = false;

    pthread_mutex_unlock(&g_isp.mutex);

    log_info("ISP shutdown complete");
}

const isp_status_t* isp_get_status(void)
{
    return &g_isp.status;
}

int isp_set_ldch_enabled(bool enabled)
{
    int ret;

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    ret = rkaiq_set_ldch_enable_stub(enabled);
    if (ret == 0) {
        g_isp.config.ldch_enabled = enabled;
        g_isp.status.ldch_enabled = enabled;
        log_info("LDCH %s", enabled ? "enabled" : "disabled");
    }

    pthread_mutex_unlock(&g_isp.mutex);
    return ret;
}

int isp_set_ldch_level(uint8_t level)
{
    int ret;

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    ret = rkaiq_set_ldch_level_stub(level);
    if (ret == 0) {
        g_isp.config.ldch_level = level;
        g_isp.status.ldch_level = level;
        log_info("LDCH level set to %d", level);
    }

    pthread_mutex_unlock(&g_isp.mutex);
    return ret;
}

int isp_set_ldch_mesh(const char* mesh_path)
{
    int ret;

    if (!mesh_path) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    ret = rkaiq_set_ldch_mesh_stub(mesh_path);
    if (ret == 0) {
        strncpy(g_isp.config.ldch_mesh_path, mesh_path,
                sizeof(g_isp.config.ldch_mesh_path) - 1);
        log_info("LDCH mesh loaded from: %s", mesh_path);
    }

    pthread_mutex_unlock(&g_isp.mutex);
    return ret;
}

int isp_set_exposure(bool auto_mode, float manual_time_ms, float manual_gain)
{
    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    // Stub: In production, use rk_aiq_uapi2_setExpMode() and related APIs
    log_info("[ISP STUB] Exposure: auto=%d, time=%.2fms, gain=%.2f",
             auto_mode, manual_time_ms, manual_gain);

    g_isp.config.auto_exposure = auto_mode;

    pthread_mutex_unlock(&g_isp.mutex);
    return 0;
}

int isp_set_white_balance(bool auto_mode, float manual_r_gain, float manual_b_gain)
{
    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    // Stub: In production, use rk_aiq_uapi2_setWBMode() and related APIs
    log_info("[ISP STUB] WB: auto=%d, r_gain=%.2f, b_gain=%.2f",
             auto_mode, manual_r_gain, manual_b_gain);

    g_isp.config.auto_white_balance = auto_mode;

    pthread_mutex_unlock(&g_isp.mutex);
    return 0;
}

int isp_get_exposure_info(isp_exposure_info_t* info)
{
    if (!info) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    // Stub: In production, query from AIQ
    info->exposure_time_ms = g_isp.status.current_exposure_time_ms;
    info->analog_gain = g_isp.status.current_gain;
    info->digital_gain = 1.0f;
    info->iso = info->analog_gain * 100.0f;

    pthread_mutex_unlock(&g_isp.mutex);
    return 0;
}

int isp_get_awb_info(isp_awb_info_t* info)
{
    if (!info) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_isp.mutex);

    if (!g_isp.status.initialized) {
        pthread_mutex_unlock(&g_isp.mutex);
        return -EINVAL;
    }

    // Stub: In production, query from AIQ
    info->r_gain = 1.0f;
    info->g_gain = 1.0f;
    info->b_gain = 1.0f;
    info->color_temp_k = g_isp.status.current_color_temp_k;

    pthread_mutex_unlock(&g_isp.mutex);
    return 0;
}

float isp_calculate_azimuth(float pixel_x, float fov_degrees)
{
    /*
     * With LDCH cylindrical projection enabled at level 255,
     * pixel X position maps linearly to azimuth angle.
     *
     * pixel_x = 0.0 -> azimuth = -fov/2 (left edge)
     * pixel_x = 0.5 -> azimuth = 0 (center)
     * pixel_x = 1.0 -> azimuth = +fov/2 (right edge)
     */
    float center_offset = pixel_x - 0.5f;  // Range: -0.5 to +0.5
    return center_offset * fov_degrees;
}

float isp_calculate_elevation(float pixel_y, float fov_degrees)
{
    /*
     * Similar linear mapping for vertical angle.
     *
     * pixel_y = 0.0 -> elevation = +fov/2 (top edge)
     * pixel_y = 0.5 -> elevation = 0 (center)
     * pixel_y = 1.0 -> elevation = -fov/2 (bottom edge)
     *
     * Note: Y is inverted (top is positive elevation)
     */
    float center_offset = 0.5f - pixel_y;  // Range: -0.5 to +0.5 (inverted)
    return center_offset * fov_degrees;
}
