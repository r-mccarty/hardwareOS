/**
 * @file vision_pipeline.h
 * @brief Vision Pipeline Orchestrator for RS-1
 *
 * Coordinates the camera → ISP → NPU pipeline:
 * 1. SC3336 MIPI camera capture (2304x1296 @ 30fps)
 * 2. Rockchip ISP with LDCH lens correction
 * 3. RGA resize to 640x640 for NPU
 * 4. YOLOv8 inference on RKNN NPU
 * 5. H.265 encoding for WebRTC video track
 *
 * Data flow:
 * ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
 * │   Camera    │───▶│     ISP     │───▶│    Fork     │
 * │   SC3336    │    │   + LDCH    │    │             │
 * └─────────────┘    └─────────────┘    └──┬──────┬───┘
 *                                          │      │
 *                    ┌─────────────────────┘      └──────────────────┐
 *                    ▼                                               ▼
 *             ┌─────────────┐                                 ┌─────────────┐
 *             │ RGA Resize  │                                 │ RGA Resize  │
 *             │   1080p     │                                 │   640x640   │
 *             └──────┬──────┘                                 └──────┬──────┘
 *                    ▼                                               ▼
 *             ┌─────────────┐                                 ┌─────────────┐
 *             │   H.265     │                                 │     NPU     │
 *             │   Encode    │                                 │   YOLOv8    │
 *             └──────┬──────┘                                 └──────┬──────┘
 *                    ▼                                               ▼
 *               VideoFrame                                     VisionFrame
 *             (via callback)                                (via callback)
 */

#ifndef RS1_VISION_PIPELINE_H
#define RS1_VISION_PIPELINE_H

#include <stdbool.h>
#include <stdint.h>
#include "camera.h"
#include "isp.h"
#include "npu.h"

#ifdef __cplusplus
extern "C" {
#endif

// Pipeline output resolutions
#define VISION_ENCODE_WIDTH     1920
#define VISION_ENCODE_HEIGHT    1080
#define VISION_NPU_WIDTH        640
#define VISION_NPU_HEIGHT       640

// H.265 encoder settings
#define VISION_ENCODER_CODEC    1       // 0=H.264, 1=H.265
#define VISION_ENCODER_BITRATE  1500    // kbps
#define VISION_ENCODER_GOP      60      // frames

// Pipeline configuration
typedef struct {
    // Camera settings
    uint32_t camera_width;
    uint32_t camera_height;
    uint32_t camera_fps;

    // ISP settings
    bool     ldch_enabled;
    uint8_t  ldch_level;

    // NPU settings
    char     model_path[256];
    float    confidence_threshold;
    float    horizontal_fov_deg;
    float    vertical_fov_deg;
    bool     person_only;

    // Encoder settings
    uint32_t encode_width;
    uint32_t encode_height;
    int32_t  bitrate_kbps;
    bool     use_h265;
} vision_pipeline_config_t;

// Pipeline status
typedef struct {
    bool     initialized;
    bool     running;

    // Component status
    bool     camera_ready;
    bool     isp_ready;
    bool     npu_ready;
    bool     encoder_ready;

    // Statistics
    uint64_t frames_captured;
    uint64_t frames_encoded;
    uint64_t frames_inferred;
    float    capture_fps;
    float    encode_fps;
    float    inference_fps;
    float    avg_inference_ms;
    float    avg_encode_ms;

    // Error tracking
    uint32_t capture_errors;
    uint32_t encode_errors;
    uint32_t inference_errors;
    char     last_error[256];
} vision_pipeline_status_t;

// Encoded video frame callback
typedef void (*vision_video_callback_t)(
    const uint8_t* data,
    size_t size,
    uint64_t timestamp_ns,
    bool is_keyframe,
    void* user_data
);

// NPU detection callback
typedef void (*vision_detection_callback_t)(
    const npu_result_t* result,
    void* user_data
);

/**
 * @brief Initialize the vision pipeline
 * @param config Pipeline configuration (NULL for defaults)
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_init(const vision_pipeline_config_t* config);

/**
 * @brief Shutdown the vision pipeline
 */
void vision_pipeline_shutdown(void);

/**
 * @brief Start the vision pipeline
 * @param video_cb Callback for encoded video frames (may be NULL)
 * @param detection_cb Callback for NPU detections (may be NULL)
 * @param user_data User data passed to callbacks
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_start(
    vision_video_callback_t video_cb,
    vision_detection_callback_t detection_cb,
    void* user_data
);

/**
 * @brief Stop the vision pipeline
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_stop(void);

/**
 * @brief Get current pipeline status
 * @return Pointer to pipeline status structure
 */
const vision_pipeline_status_t* vision_pipeline_get_status(void);

/**
 * @brief Set video encoding bitrate
 * @param bitrate_kbps Target bitrate in kbps
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_set_bitrate(int32_t bitrate_kbps);

/**
 * @brief Request a keyframe
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_request_keyframe(void);

/**
 * @brief Enable or disable NPU inference
 * @param enabled Enable NPU processing
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_set_npu_enabled(bool enabled);

/**
 * @brief Update LDCH settings
 * @param enabled Enable LDCH
 * @param level Correction level (0-255)
 * @return 0 on success, negative error code on failure
 */
int vision_pipeline_set_ldch(bool enabled, uint8_t level);

/**
 * @brief Get current configuration
 * @return Pointer to current configuration
 */
const vision_pipeline_config_t* vision_pipeline_get_config(void);

#ifdef __cplusplus
}
#endif

#endif // RS1_VISION_PIPELINE_H
