/**
 * @file isp.h
 * @brief Rockchip ISP Interface for RS-1 Vision Pipeline
 *
 * Configures the Rockchip ISP (Image Signal Processor) with:
 * - 3A algorithms (Auto Exposure, Auto White Balance, Auto Focus)
 * - LDCH (Lens Distortion Correction for Horizontal lines)
 * - Noise reduction
 * - HDR processing (optional)
 */

#ifndef RS1_ISP_H
#define RS1_ISP_H

#include <stdbool.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// ISP configuration paths
#define ISP_IQ_FILES_PATH   "/etc/iqfiles"
#define ISP_LDCH_MESH_PATH  "/userdata/ldch_mesh.bin"

// LDCH correction level range: 0-255
// 255 = full cylindrical projection for linear pixel-to-angle mapping
#define ISP_LDCH_LEVEL_OFF      0
#define ISP_LDCH_LEVEL_LOW      64
#define ISP_LDCH_LEVEL_MEDIUM   128
#define ISP_LDCH_LEVEL_HIGH     192
#define ISP_LDCH_LEVEL_FULL     255

// ISP configuration
typedef struct {
    bool     ldch_enabled;
    uint8_t  ldch_level;        // 0-255 correction strength
    char     ldch_mesh_path[256];
    bool     hdr_enabled;
    bool     noise_reduction;
    bool     auto_exposure;
    bool     auto_white_balance;
} isp_config_t;

// ISP status
typedef struct {
    bool     initialized;
    bool     running;
    bool     ldch_enabled;
    uint8_t  ldch_level;
    float    current_exposure_time_ms;
    float    current_gain;
    int32_t  current_color_temp_k;
} isp_status_t;

// Exposure info (from 3A)
typedef struct {
    float    exposure_time_ms;
    float    analog_gain;
    float    digital_gain;
    float    iso;
} isp_exposure_info_t;

// White balance info (from 3A)
typedef struct {
    float    r_gain;
    float    g_gain;
    float    b_gain;
    int32_t  color_temp_k;
} isp_awb_info_t;

/**
 * @brief Initialize the Rockchip ISP
 * @param sensor_name Sensor name for IQ file lookup (e.g., "sc3336")
 * @param config ISP configuration (NULL for defaults)
 * @return 0 on success, negative error code on failure
 */
int isp_init(const char* sensor_name, const isp_config_t* config);

/**
 * @brief Start ISP processing (3A, LDCH, etc.)
 * @return 0 on success, negative error code on failure
 */
int isp_start(void);

/**
 * @brief Stop ISP processing
 * @return 0 on success, negative error code on failure
 */
int isp_stop(void);

/**
 * @brief Shutdown ISP subsystem
 */
void isp_shutdown(void);

/**
 * @brief Get current ISP status
 * @return Pointer to ISP status structure
 */
const isp_status_t* isp_get_status(void);

/**
 * @brief Enable or disable LDCH lens correction
 * @param enabled Enable LDCH
 * @return 0 on success, negative error code on failure
 */
int isp_set_ldch_enabled(bool enabled);

/**
 * @brief Set LDCH correction level
 * @param level Correction level (0-255, 255=full cylindrical projection)
 * @return 0 on success, negative error code on failure
 */
int isp_set_ldch_level(uint8_t level);

/**
 * @brief Load custom LDCH mesh for lens calibration
 * @param mesh_path Path to mesh file containing pixel displacement vectors
 * @return 0 on success, negative error code on failure
 */
int isp_set_ldch_mesh(const char* mesh_path);

/**
 * @brief Set exposure mode
 * @param auto_mode Enable auto exposure
 * @param manual_time_ms Manual exposure time in ms (ignored if auto)
 * @param manual_gain Manual gain value (ignored if auto)
 * @return 0 on success, negative error code on failure
 */
int isp_set_exposure(bool auto_mode, float manual_time_ms, float manual_gain);

/**
 * @brief Set white balance mode
 * @param auto_mode Enable auto white balance
 * @param manual_r_gain Manual red gain (ignored if auto)
 * @param manual_b_gain Manual blue gain (ignored if auto)
 * @return 0 on success, negative error code on failure
 */
int isp_set_white_balance(bool auto_mode, float manual_r_gain, float manual_b_gain);

/**
 * @brief Get current exposure information
 * @param info Pointer to exposure info structure
 * @return 0 on success, negative error code on failure
 */
int isp_get_exposure_info(isp_exposure_info_t* info);

/**
 * @brief Get current white balance information
 * @param info Pointer to AWB info structure
 * @return 0 on success, negative error code on failure
 */
int isp_get_awb_info(isp_awb_info_t* info);

/**
 * @brief Calculate azimuth angle from pixel X position
 *
 * With LDCH cylindrical projection enabled, pixel X position maps
 * linearly to azimuth angle.
 *
 * @param pixel_x Normalized X position (0.0 to 1.0)
 * @param fov_degrees Horizontal field of view in degrees
 * @return Azimuth angle in degrees from center (-fov/2 to +fov/2)
 */
float isp_calculate_azimuth(float pixel_x, float fov_degrees);

/**
 * @brief Calculate elevation angle from pixel Y position
 * @param pixel_y Normalized Y position (0.0 to 1.0)
 * @param fov_degrees Vertical field of view in degrees
 * @return Elevation angle in degrees from center (-fov/2 to +fov/2)
 */
float isp_calculate_elevation(float pixel_y, float fov_degrees);

#ifdef __cplusplus
}
#endif

#endif // RS1_ISP_H
