/**
 * @file vision_pipeline.c
 * @brief Vision Pipeline Orchestrator for RS-1
 *
 * Coordinates the full vision pipeline from camera capture through
 * ISP processing, NPU inference, and video encoding.
 *
 * Note: This is a spike implementation. RGA (Rockchip Graphics Accelerator)
 * and MPP encoder integration require the Rockchip SDK libraries.
 */

#define _POSIX_C_SOURCE 200809L

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <pthread.h>
#include <time.h>
#include <unistd.h>

#include "vision_pipeline.h"
#include "camera.h"
#include "isp.h"
#include "npu.h"
#include "log.h"

// Frame buffer sizes
#define MAX_NV12_SIZE   (CAMERA_SENSOR_WIDTH * CAMERA_SENSOR_HEIGHT * 3 / 2)
#define MAX_RGB_SIZE    (VISION_NPU_WIDTH * VISION_NPU_HEIGHT * 3)
#define MAX_ENCODE_SIZE (VISION_ENCODE_WIDTH * VISION_ENCODE_HEIGHT * 3 / 2)

// Module state
static struct {
    vision_pipeline_config_t    config;
    vision_pipeline_status_t    status;

    // Callbacks
    vision_video_callback_t     video_callback;
    vision_detection_callback_t detection_callback;
    void*                       user_data;

    // Processing thread
    pthread_t                   process_thread;
    volatile bool               running;
    pthread_mutex_t             mutex;

    // Frame buffers
    uint8_t*                    npu_rgb_buffer;    // RGB for NPU input
    uint8_t*                    encode_buffer;     // NV12 for encoder

    // Timing
    uint64_t                    last_capture_time;
    uint64_t                    last_encode_time;
    uint64_t                    last_inference_time;
    uint64_t                    total_encode_time_us;
    uint64_t                    total_inference_time_us;

    // NPU control
    bool                        npu_enabled;
    int                         npu_skip_count;    // Skip frames for NPU
    int                         npu_skip_current;
} g_pipeline = {
    .running = false,
    .mutex = PTHREAD_MUTEX_INITIALIZER,
    .npu_enabled = true,
    .npu_skip_count = 0,  // Process every frame
    .npu_skip_current = 0,
};

static uint64_t get_timestamp_us(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000ULL + (uint64_t)ts.tv_nsec / 1000ULL;
}

static uint64_t get_timestamp_ns(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}

// Stub: RGA resize NV12 to smaller NV12
static int rga_resize_nv12(const uint8_t* src, int src_w, int src_h,
                            uint8_t* dst, int dst_w, int dst_h)
{
    (void)src;
    (void)src_w;
    (void)src_h;
    (void)dst;
    (void)dst_w;
    (void)dst_h;
    log_trace("[RGA STUB] Resize NV12 %dx%d -> %dx%d", src_w, src_h, dst_w, dst_h);
    // In production: Use RGA2 hardware acceleration
    return 0;
}

// Stub: RGA convert NV12 to RGB
static int rga_nv12_to_rgb(const uint8_t* nv12, int width, int height,
                            uint8_t* rgb, int rgb_w, int rgb_h)
{
    (void)nv12;
    (void)width;
    (void)height;
    (void)rgb;
    (void)rgb_w;
    (void)rgb_h;
    log_trace("[RGA STUB] NV12 %dx%d -> RGB %dx%d", width, height, rgb_w, rgb_h);
    // In production: Use RGA2 for fast color conversion
    return 0;
}

// Stub: Encode frame to H.265
static int encode_frame(const uint8_t* nv12, int width, int height,
                        uint8_t* output, size_t* output_size,
                        bool* is_keyframe)
{
    (void)nv12;
    (void)width;
    (void)height;
    log_trace("[VENC STUB] Encode %dx%d", width, height);

    // Simulate encoder output (just for testing)
    *output_size = 1024;  // Fake size
    *is_keyframe = (g_pipeline.status.frames_encoded % 60 == 0);

    // In production: Use RK_MPI_VENC
    return 0;
}

// Camera frame callback - runs in capture thread
static void on_camera_frame(const camera_frame_t* frame, void* user_data)
{
    (void)user_data;

    uint64_t now = get_timestamp_us();
    uint64_t start_time = now;

    pthread_mutex_lock(&g_pipeline.mutex);

    if (!g_pipeline.running) {
        pthread_mutex_unlock(&g_pipeline.mutex);
        return;
    }

    g_pipeline.status.frames_captured++;

    // Calculate capture FPS
    if (g_pipeline.last_capture_time > 0) {
        uint64_t delta = now - g_pipeline.last_capture_time;
        if (delta > 0) {
            float instant_fps = 1000000.0f / (float)delta;
            g_pipeline.status.capture_fps =
                0.9f * g_pipeline.status.capture_fps + 0.1f * instant_fps;
        }
    }
    g_pipeline.last_capture_time = now;

    // Fork 1: Resize for encoder and encode
    if (g_pipeline.video_callback) {
        uint64_t encode_start = get_timestamp_us();

        // Resize to encode resolution
        rga_resize_nv12(frame->data, frame->width, frame->height,
                        g_pipeline.encode_buffer,
                        VISION_ENCODE_WIDTH, VISION_ENCODE_HEIGHT);

        // Encode to H.265
        uint8_t* encoded_output = g_pipeline.encode_buffer;  // Reuse buffer
        size_t encoded_size = 0;
        bool is_keyframe = false;

        int ret = encode_frame(g_pipeline.encode_buffer,
                               VISION_ENCODE_WIDTH, VISION_ENCODE_HEIGHT,
                               encoded_output, &encoded_size, &is_keyframe);

        uint64_t encode_end = get_timestamp_us();
        uint64_t encode_time = encode_end - encode_start;
        g_pipeline.total_encode_time_us += encode_time;

        if (ret == 0 && encoded_size > 0) {
            g_pipeline.status.frames_encoded++;
            g_pipeline.status.avg_encode_ms =
                (float)g_pipeline.total_encode_time_us /
                (float)g_pipeline.status.frames_encoded / 1000.0f;

            // Calculate encode FPS
            if (g_pipeline.last_encode_time > 0) {
                uint64_t delta = encode_end - g_pipeline.last_encode_time;
                if (delta > 0) {
                    float instant_fps = 1000000.0f / (float)delta;
                    g_pipeline.status.encode_fps =
                        0.9f * g_pipeline.status.encode_fps + 0.1f * instant_fps;
                }
            }
            g_pipeline.last_encode_time = encode_end;

            // Deliver encoded frame via callback (outside mutex)
            vision_video_callback_t cb = g_pipeline.video_callback;
            void* ud = g_pipeline.user_data;
            pthread_mutex_unlock(&g_pipeline.mutex);

            cb(encoded_output, encoded_size, frame->timestamp_ns, is_keyframe, ud);

            pthread_mutex_lock(&g_pipeline.mutex);
        } else {
            g_pipeline.status.encode_errors++;
        }
    }

    // Fork 2: NPU inference (if enabled)
    if (g_pipeline.npu_enabled && g_pipeline.detection_callback) {
        // Skip frames to reduce NPU load if configured
        if (g_pipeline.npu_skip_current > 0) {
            g_pipeline.npu_skip_current--;
        } else {
            g_pipeline.npu_skip_current = g_pipeline.npu_skip_count;

            uint64_t infer_start = get_timestamp_us();

            // Convert NV12 to RGB and resize for NPU
            rga_nv12_to_rgb(frame->data, frame->width, frame->height,
                            g_pipeline.npu_rgb_buffer,
                            VISION_NPU_WIDTH, VISION_NPU_HEIGHT);

            // Run inference
            npu_result_t result;
            int ret = npu_inference(g_pipeline.npu_rgb_buffer,
                                    VISION_NPU_WIDTH, VISION_NPU_HEIGHT,
                                    &result);

            uint64_t infer_end = get_timestamp_us();
            uint64_t infer_time = infer_end - infer_start;
            g_pipeline.total_inference_time_us += infer_time;

            if (ret == 0) {
                g_pipeline.status.frames_inferred++;
                g_pipeline.status.avg_inference_ms =
                    (float)g_pipeline.total_inference_time_us /
                    (float)g_pipeline.status.frames_inferred / 1000.0f;

                // Calculate inference FPS
                if (g_pipeline.last_inference_time > 0) {
                    uint64_t delta = infer_end - g_pipeline.last_inference_time;
                    if (delta > 0) {
                        float instant_fps = 1000000.0f / (float)delta;
                        g_pipeline.status.inference_fps =
                            0.9f * g_pipeline.status.inference_fps + 0.1f * instant_fps;
                    }
                }
                g_pipeline.last_inference_time = infer_end;

                // Deliver detections via callback (outside mutex)
                vision_detection_callback_t cb = g_pipeline.detection_callback;
                void* ud = g_pipeline.user_data;
                pthread_mutex_unlock(&g_pipeline.mutex);

                cb(&result, ud);

                pthread_mutex_lock(&g_pipeline.mutex);
            } else {
                g_pipeline.status.inference_errors++;
            }
        }
    }

    pthread_mutex_unlock(&g_pipeline.mutex);

    uint64_t total_time = get_timestamp_us() - start_time;
    log_trace("Frame processed in %lu us", (unsigned long)total_time);
}

int vision_pipeline_init(const vision_pipeline_config_t* config)
{
    int ret;

    pthread_mutex_lock(&g_pipeline.mutex);

    if (g_pipeline.status.initialized) {
        log_warn("Vision pipeline already initialized");
        pthread_mutex_unlock(&g_pipeline.mutex);
        return 0;
    }

    // Apply configuration
    if (config) {
        g_pipeline.config = *config;
    } else {
        // Default configuration
        g_pipeline.config.camera_width = CAMERA_SENSOR_WIDTH;
        g_pipeline.config.camera_height = CAMERA_SENSOR_HEIGHT;
        g_pipeline.config.camera_fps = CAMERA_SENSOR_FPS;
        g_pipeline.config.ldch_enabled = true;
        g_pipeline.config.ldch_level = ISP_LDCH_LEVEL_FULL;
        strcpy(g_pipeline.config.model_path, NPU_DEFAULT_MODEL_PATH);
        g_pipeline.config.confidence_threshold = 0.5f;
        g_pipeline.config.horizontal_fov_deg = 120.0f;
        g_pipeline.config.vertical_fov_deg = 90.0f;
        g_pipeline.config.person_only = false;
        g_pipeline.config.encode_width = VISION_ENCODE_WIDTH;
        g_pipeline.config.encode_height = VISION_ENCODE_HEIGHT;
        g_pipeline.config.bitrate_kbps = VISION_ENCODER_BITRATE;
        g_pipeline.config.use_h265 = true;
    }

    log_info("Initializing vision pipeline:");
    log_info("  Camera: %dx%d @ %d fps",
             g_pipeline.config.camera_width,
             g_pipeline.config.camera_height,
             g_pipeline.config.camera_fps);
    log_info("  Encode: %dx%d @ %d kbps",
             g_pipeline.config.encode_width,
             g_pipeline.config.encode_height,
             g_pipeline.config.bitrate_kbps);
    log_info("  NPU: %dx%d, model: %s",
             VISION_NPU_WIDTH, VISION_NPU_HEIGHT,
             g_pipeline.config.model_path);

    // Allocate buffers
    g_pipeline.npu_rgb_buffer = (uint8_t*)malloc(MAX_RGB_SIZE);
    g_pipeline.encode_buffer = (uint8_t*)malloc(MAX_ENCODE_SIZE);

    if (!g_pipeline.npu_rgb_buffer || !g_pipeline.encode_buffer) {
        log_error("Failed to allocate processing buffers");
        if (g_pipeline.npu_rgb_buffer) free(g_pipeline.npu_rgb_buffer);
        if (g_pipeline.encode_buffer) free(g_pipeline.encode_buffer);
        g_pipeline.npu_rgb_buffer = NULL;
        g_pipeline.encode_buffer = NULL;
        pthread_mutex_unlock(&g_pipeline.mutex);
        return -ENOMEM;
    }

    // Initialize camera
    camera_config_t cam_cfg = {
        .width = g_pipeline.config.camera_width,
        .height = g_pipeline.config.camera_height,
        .fps = g_pipeline.config.camera_fps,
        .hdr_enabled = false,
        .mirror = false,
        .flip = false,
    };

    ret = camera_init(&cam_cfg);
    if (ret < 0) {
        log_error("Camera init failed: %d", ret);
        strncpy(g_pipeline.status.last_error, "Camera init failed",
                sizeof(g_pipeline.status.last_error) - 1);
        // Continue anyway for spike - camera may not exist on dev machine
    } else {
        g_pipeline.status.camera_ready = true;
    }

    // Initialize ISP
    isp_config_t isp_cfg = {
        .ldch_enabled = g_pipeline.config.ldch_enabled,
        .ldch_level = g_pipeline.config.ldch_level,
        .hdr_enabled = false,
        .noise_reduction = true,
        .auto_exposure = true,
        .auto_white_balance = true,
    };

    ret = isp_init("sc3336", &isp_cfg);
    if (ret < 0) {
        log_warn("ISP init failed: %d (continuing)", ret);
    } else {
        g_pipeline.status.isp_ready = true;
    }

    // Initialize NPU
    npu_config_t npu_cfg = {
        .confidence_threshold = g_pipeline.config.confidence_threshold,
        .nms_threshold = 0.45f,
        .horizontal_fov_deg = g_pipeline.config.horizontal_fov_deg,
        .vertical_fov_deg = g_pipeline.config.vertical_fov_deg,
        .person_only = g_pipeline.config.person_only,
    };
    strncpy(npu_cfg.model_path, g_pipeline.config.model_path,
            sizeof(npu_cfg.model_path) - 1);

    ret = npu_init(&npu_cfg);
    if (ret < 0) {
        log_warn("NPU init failed: %d (continuing)", ret);
    } else {
        g_pipeline.status.npu_ready = true;
    }

    // Encoder init would go here (stub for now)
    g_pipeline.status.encoder_ready = true;

    g_pipeline.status.initialized = true;
    g_pipeline.npu_enabled = true;

    pthread_mutex_unlock(&g_pipeline.mutex);

    log_info("Vision pipeline initialized (camera=%d, isp=%d, npu=%d, enc=%d)",
             g_pipeline.status.camera_ready,
             g_pipeline.status.isp_ready,
             g_pipeline.status.npu_ready,
             g_pipeline.status.encoder_ready);

    return 0;
}

void vision_pipeline_shutdown(void)
{
    pthread_mutex_lock(&g_pipeline.mutex);

    if (!g_pipeline.status.initialized) {
        pthread_mutex_unlock(&g_pipeline.mutex);
        return;
    }

    // Stop pipeline if running
    if (g_pipeline.status.running) {
        pthread_mutex_unlock(&g_pipeline.mutex);
        vision_pipeline_stop();
        pthread_mutex_lock(&g_pipeline.mutex);
    }

    // Shutdown components
    npu_shutdown();
    isp_shutdown();
    camera_shutdown();

    // Free buffers
    if (g_pipeline.npu_rgb_buffer) {
        free(g_pipeline.npu_rgb_buffer);
        g_pipeline.npu_rgb_buffer = NULL;
    }
    if (g_pipeline.encode_buffer) {
        free(g_pipeline.encode_buffer);
        g_pipeline.encode_buffer = NULL;
    }

    // Reset status
    memset(&g_pipeline.status, 0, sizeof(g_pipeline.status));

    pthread_mutex_unlock(&g_pipeline.mutex);

    log_info("Vision pipeline shutdown complete");
}

int vision_pipeline_start(
    vision_video_callback_t video_cb,
    vision_detection_callback_t detection_cb,
    void* user_data)
{
    int ret;

    pthread_mutex_lock(&g_pipeline.mutex);

    if (!g_pipeline.status.initialized) {
        log_error("Vision pipeline not initialized");
        pthread_mutex_unlock(&g_pipeline.mutex);
        return -EINVAL;
    }

    if (g_pipeline.status.running) {
        log_warn("Vision pipeline already running");
        pthread_mutex_unlock(&g_pipeline.mutex);
        return 0;
    }

    g_pipeline.video_callback = video_cb;
    g_pipeline.detection_callback = detection_cb;
    g_pipeline.user_data = user_data;
    g_pipeline.running = true;

    // Reset statistics
    g_pipeline.status.frames_captured = 0;
    g_pipeline.status.frames_encoded = 0;
    g_pipeline.status.frames_inferred = 0;
    g_pipeline.status.capture_fps = 0;
    g_pipeline.status.encode_fps = 0;
    g_pipeline.status.inference_fps = 0;
    g_pipeline.total_encode_time_us = 0;
    g_pipeline.total_inference_time_us = 0;
    g_pipeline.last_capture_time = 0;
    g_pipeline.last_encode_time = 0;
    g_pipeline.last_inference_time = 0;

    pthread_mutex_unlock(&g_pipeline.mutex);

    // Start ISP
    ret = isp_start();
    if (ret < 0) {
        log_warn("ISP start failed: %d (continuing)", ret);
    }

    // Start camera streaming
    ret = camera_start_streaming(on_camera_frame, NULL);
    if (ret < 0) {
        log_error("Camera start failed: %d", ret);
        pthread_mutex_lock(&g_pipeline.mutex);
        g_pipeline.running = false;
        pthread_mutex_unlock(&g_pipeline.mutex);
        return ret;
    }

    pthread_mutex_lock(&g_pipeline.mutex);
    g_pipeline.status.running = true;
    pthread_mutex_unlock(&g_pipeline.mutex);

    log_info("Vision pipeline started");
    return 0;
}

int vision_pipeline_stop(void)
{
    pthread_mutex_lock(&g_pipeline.mutex);

    if (!g_pipeline.status.running) {
        pthread_mutex_unlock(&g_pipeline.mutex);
        return 0;
    }

    g_pipeline.running = false;
    pthread_mutex_unlock(&g_pipeline.mutex);

    // Stop camera streaming
    camera_stop_streaming();

    // Stop ISP
    isp_stop();

    pthread_mutex_lock(&g_pipeline.mutex);
    g_pipeline.status.running = false;
    g_pipeline.video_callback = NULL;
    g_pipeline.detection_callback = NULL;
    pthread_mutex_unlock(&g_pipeline.mutex);

    log_info("Vision pipeline stopped");
    log_info("  Frames: captured=%lu, encoded=%lu, inferred=%lu",
             (unsigned long)g_pipeline.status.frames_captured,
             (unsigned long)g_pipeline.status.frames_encoded,
             (unsigned long)g_pipeline.status.frames_inferred);
    log_info("  Avg times: encode=%.1fms, inference=%.1fms",
             g_pipeline.status.avg_encode_ms,
             g_pipeline.status.avg_inference_ms);

    return 0;
}

const vision_pipeline_status_t* vision_pipeline_get_status(void)
{
    return &g_pipeline.status;
}

int vision_pipeline_set_bitrate(int32_t bitrate_kbps)
{
    pthread_mutex_lock(&g_pipeline.mutex);
    g_pipeline.config.bitrate_kbps = bitrate_kbps;
    // In production: Update MPP encoder bitrate
    log_info("Bitrate set to %d kbps", bitrate_kbps);
    pthread_mutex_unlock(&g_pipeline.mutex);
    return 0;
}

int vision_pipeline_request_keyframe(void)
{
    // In production: Request IDR frame from encoder
    log_info("Keyframe requested");
    return 0;
}

int vision_pipeline_set_npu_enabled(bool enabled)
{
    pthread_mutex_lock(&g_pipeline.mutex);
    g_pipeline.npu_enabled = enabled;
    log_info("NPU %s", enabled ? "enabled" : "disabled");
    pthread_mutex_unlock(&g_pipeline.mutex);
    return 0;
}

int vision_pipeline_set_ldch(bool enabled, uint8_t level)
{
    int ret;

    ret = isp_set_ldch_enabled(enabled);
    if (ret == 0 && enabled) {
        ret = isp_set_ldch_level(level);
    }

    pthread_mutex_lock(&g_pipeline.mutex);
    g_pipeline.config.ldch_enabled = enabled;
    g_pipeline.config.ldch_level = level;
    pthread_mutex_unlock(&g_pipeline.mutex);

    return ret;
}

const vision_pipeline_config_t* vision_pipeline_get_config(void)
{
    return &g_pipeline.config;
}
