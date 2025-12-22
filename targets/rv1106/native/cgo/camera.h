/**
 * @file camera.h
 * @brief SC3336 MIPI Camera Interface for RS-1 Vision Pipeline
 *
 * Initializes and controls the SC3336 3MP MIPI CSI-2 sensor.
 * Resolution: 2304x1296 @ 30fps, RAW Bayer (RGGB)
 */

#ifndef RS1_CAMERA_H
#define RS1_CAMERA_H

#include <stdbool.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// SC3336 sensor specifications
#define CAMERA_SENSOR_WIDTH  2304
#define CAMERA_SENSOR_HEIGHT 1296
#define CAMERA_SENSOR_FPS    30

// Camera device paths (RV1106 specific)
#define CAMERA_MIPI_DEV     "/dev/video0"   // MIPI CSI input
#define CAMERA_ISP_DEV      "/dev/video1"   // ISP output (NV12)
#define CAMERA_SENSOR_DEV   "/dev/v4l-subdev0"

// Camera configuration
typedef struct {
    uint32_t width;
    uint32_t height;
    uint32_t fps;
    bool     hdr_enabled;
    bool     mirror;
    bool     flip;
} camera_config_t;

// Camera status
typedef struct {
    bool     initialized;
    bool     streaming;
    uint32_t width;
    uint32_t height;
    uint32_t fps;
    uint64_t frame_count;
    uint64_t last_frame_ts_ns;
} camera_status_t;

// Frame buffer for camera capture
typedef struct {
    uint8_t* data;        // NV12 buffer from ISP
    size_t   size;
    uint32_t width;
    uint32_t height;
    int      dma_fd;      // DMA buffer file descriptor
    uint64_t timestamp_ns;
    uint32_t sequence;
} camera_frame_t;

// Callback for frame delivery
typedef void (*camera_frame_callback_t)(const camera_frame_t* frame, void* user_data);

/**
 * @brief Initialize the SC3336 MIPI camera
 * @param config Camera configuration (NULL for defaults)
 * @return 0 on success, negative error code on failure
 */
int camera_init(const camera_config_t* config);

/**
 * @brief Shutdown the camera subsystem
 */
void camera_shutdown(void);

/**
 * @brief Start camera streaming
 * @param callback Frame callback function
 * @param user_data User data passed to callback
 * @return 0 on success, negative error code on failure
 */
int camera_start_streaming(camera_frame_callback_t callback, void* user_data);

/**
 * @brief Stop camera streaming
 * @return 0 on success, negative error code on failure
 */
int camera_stop_streaming(void);

/**
 * @brief Get current camera status
 * @return Pointer to camera status structure
 */
const camera_status_t* camera_get_status(void);

/**
 * @brief Set camera exposure mode
 * @param auto_exposure Enable auto exposure
 * @param manual_value Manual exposure value (ignored if auto)
 * @return 0 on success, negative error code on failure
 */
int camera_set_exposure(bool auto_exposure, int32_t manual_value);

/**
 * @brief Set camera gain
 * @param auto_gain Enable auto gain
 * @param manual_value Manual gain value (ignored if auto)
 * @return 0 on success, negative error code on failure
 */
int camera_set_gain(bool auto_gain, int32_t manual_value);

/**
 * @brief Set camera mirror/flip
 * @param mirror Enable horizontal mirror
 * @param flip Enable vertical flip
 * @return 0 on success, negative error code on failure
 */
int camera_set_orientation(bool mirror, bool flip);

#ifdef __cplusplus
}
#endif

#endif // RS1_CAMERA_H
