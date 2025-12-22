/**
 * @file npu.c
 * @brief Rockchip NPU Implementation for RS-1 Vision Pipeline
 *
 * Implements YOLOv8 object detection using RKNN runtime.
 * Handles model loading, inference, and YOLO output post-processing.
 *
 * Note: This is a spike implementation. Full implementation requires
 * the RKNN runtime library from the Rockchip SDK.
 */

#define _POSIX_C_SOURCE 200809L

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <pthread.h>
#include <math.h>
#include <time.h>

#include "npu.h"
#include "isp.h"  // For azimuth/elevation calculation
#include "log.h"

// Forward declarations for RKNN types (spike - actual headers from SDK)
// In production, include: <rknn_api.h>
typedef uint64_t rknn_context;

// COCO class names (80 classes)
static const char* COCO_CLASSES[] = {
    "person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck",
    "boat", "traffic light", "fire hydrant", "stop sign", "parking meter", "bench",
    "bird", "cat", "dog", "horse", "sheep", "cow", "elephant", "bear", "zebra",
    "giraffe", "backpack", "umbrella", "handbag", "tie", "suitcase", "frisbee",
    "skis", "snowboard", "sports ball", "kite", "baseball bat", "baseball glove",
    "skateboard", "surfboard", "tennis racket", "bottle", "wine glass", "cup",
    "fork", "knife", "spoon", "bowl", "banana", "apple", "sandwich", "orange",
    "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair", "couch",
    "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse",
    "remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink",
    "refrigerator", "book", "clock", "vase", "scissors", "teddy bear", "hair drier",
    "toothbrush"
};
#define NUM_COCO_CLASSES 80

// Module state
static struct {
    rknn_context        ctx;
    bool                ctx_valid;
    npu_config_t        config;
    npu_status_t        status;
    uint8_t*            input_buffer;   // RGB input buffer
    float*              output_buffer;  // Decoded output buffer
    pthread_mutex_t     mutex;
    uint64_t            total_inference_time_us;
} g_npu = {
    .ctx = 0,
    .ctx_valid = false,
    .input_buffer = NULL,
    .output_buffer = NULL,
    .mutex = PTHREAD_MUTEX_INITIALIZER,
    .total_inference_time_us = 0,
};

static uint64_t get_timestamp_ns(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}

static uint64_t get_timestamp_us(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000ULL + (uint64_t)ts.tv_nsec / 1000ULL;
}

// Stub RKNN API implementations
static int rknn_init_stub(rknn_context* ctx, void* model_data, size_t model_size)
{
    (void)model_data;
    (void)model_size;
    log_info("[NPU STUB] rknn_init called, model size: %zu bytes", model_size);
    *ctx = 1;  // Fake context
    return 0;
}

static int rknn_destroy_stub(rknn_context ctx)
{
    (void)ctx;
    log_info("[NPU STUB] rknn_destroy called");
    return 0;
}

static int rknn_run_stub(rknn_context ctx)
{
    (void)ctx;
    // Simulate inference time (~30ms for YOLOv8n on RV1106)
    // In production, this blocks until NPU completes
    return 0;
}

// YOLOv8 output post-processing
static int yolo_postprocess(float* output, int output_size,
                            npu_detection_t* detections, int max_detections,
                            float conf_threshold, float nms_threshold)
{
    (void)output;
    (void)output_size;
    (void)nms_threshold;

    // Stub: Generate fake detections for testing
    // In production, parse YOLO output tensors and apply NMS
    int count = 0;

    // Simulate detecting a person in the center
    if (count < max_detections && conf_threshold <= 0.8f) {
        detections[count].track_id = 0;
        detections[count].class_id = YOLO_CLASS_PERSON;
        detections[count].confidence = 0.85f;
        detections[count].bbox_x = 0.5f;
        detections[count].bbox_y = 0.5f;
        detections[count].bbox_width = 0.2f;
        detections[count].bbox_height = 0.4f;
        detections[count].azimuth_deg = 0.0f;
        detections[count].elevation_deg = 0.0f;
        count++;
    }

    return count;
}

// Convert NV12 to RGB
static void nv12_to_rgb(const uint8_t* nv12, int width, int height, uint8_t* rgb)
{
    const uint8_t* y_plane = nv12;
    const uint8_t* uv_plane = nv12 + width * height;

    for (int j = 0; j < height; j++) {
        for (int i = 0; i < width; i++) {
            int y_idx = j * width + i;
            int uv_idx = (j / 2) * width + (i / 2) * 2;

            int y = y_plane[y_idx];
            int u = uv_plane[uv_idx] - 128;
            int v = uv_plane[uv_idx + 1] - 128;

            int r = y + ((v * 359) >> 8);
            int g = y - ((u * 88 + v * 183) >> 8);
            int b = y + ((u * 454) >> 8);

            // Clamp to [0, 255]
            r = r < 0 ? 0 : (r > 255 ? 255 : r);
            g = g < 0 ? 0 : (g > 255 ? 255 : g);
            b = b < 0 ? 0 : (b > 255 ? 255 : b);

            int rgb_idx = (j * width + i) * 3;
            rgb[rgb_idx] = (uint8_t)r;
            rgb[rgb_idx + 1] = (uint8_t)g;
            rgb[rgb_idx + 2] = (uint8_t)b;
        }
    }
}

// Resize RGB image using bilinear interpolation
static void resize_rgb(const uint8_t* src, int src_w, int src_h,
                       uint8_t* dst, int dst_w, int dst_h)
{
    float x_ratio = (float)src_w / dst_w;
    float y_ratio = (float)src_h / dst_h;

    for (int j = 0; j < dst_h; j++) {
        for (int i = 0; i < dst_w; i++) {
            float src_x = i * x_ratio;
            float src_y = j * y_ratio;

            int x0 = (int)src_x;
            int y0 = (int)src_y;
            int x1 = x0 + 1 < src_w ? x0 + 1 : x0;
            int y1 = y0 + 1 < src_h ? y0 + 1 : y0;

            float x_frac = src_x - x0;
            float y_frac = src_y - y0;

            for (int c = 0; c < 3; c++) {
                float p00 = src[(y0 * src_w + x0) * 3 + c];
                float p01 = src[(y0 * src_w + x1) * 3 + c];
                float p10 = src[(y1 * src_w + x0) * 3 + c];
                float p11 = src[(y1 * src_w + x1) * 3 + c];

                float val = p00 * (1 - x_frac) * (1 - y_frac) +
                           p01 * x_frac * (1 - y_frac) +
                           p10 * (1 - x_frac) * y_frac +
                           p11 * x_frac * y_frac;

                dst[(j * dst_w + i) * 3 + c] = (uint8_t)(val + 0.5f);
            }
        }
    }
}

int npu_init(const npu_config_t* config)
{
    pthread_mutex_lock(&g_npu.mutex);

    if (g_npu.status.initialized) {
        log_warn("NPU already initialized");
        pthread_mutex_unlock(&g_npu.mutex);
        return 0;
    }

    // Apply configuration
    if (config) {
        g_npu.config = *config;
    } else {
        // Default configuration
        strcpy(g_npu.config.model_path, NPU_DEFAULT_MODEL_PATH);
        g_npu.config.confidence_threshold = 0.5f;
        g_npu.config.nms_threshold = 0.45f;
        g_npu.config.horizontal_fov_deg = 120.0f;  // Wide-angle lens
        g_npu.config.vertical_fov_deg = 90.0f;
        g_npu.config.person_only = false;
    }

    log_info("Initializing NPU with model: %s", g_npu.config.model_path);
    log_info("  Confidence threshold: %.2f", g_npu.config.confidence_threshold);
    log_info("  FOV: %.1f x %.1f degrees",
             g_npu.config.horizontal_fov_deg, g_npu.config.vertical_fov_deg);

    // Allocate input buffer
    size_t input_size = NPU_INPUT_WIDTH * NPU_INPUT_HEIGHT * NPU_INPUT_CHANNELS;
    g_npu.input_buffer = (uint8_t*)malloc(input_size);
    if (!g_npu.input_buffer) {
        log_error("Failed to allocate input buffer");
        pthread_mutex_unlock(&g_npu.mutex);
        return -ENOMEM;
    }

    // Allocate output buffer (for decoded YOLO output)
    g_npu.output_buffer = (float*)malloc(NPU_MAX_DETECTIONS * 6 * sizeof(float));
    if (!g_npu.output_buffer) {
        log_error("Failed to allocate output buffer");
        free(g_npu.input_buffer);
        g_npu.input_buffer = NULL;
        pthread_mutex_unlock(&g_npu.mutex);
        return -ENOMEM;
    }

    g_npu.status.initialized = true;
    g_npu.status.model_loaded = false;
    g_npu.status.input_width = NPU_INPUT_WIDTH;
    g_npu.status.input_height = NPU_INPUT_HEIGHT;
    g_npu.status.inference_count = 0;
    g_npu.status.avg_inference_ms = 0;

    pthread_mutex_unlock(&g_npu.mutex);

    // Load default model
    if (strlen(g_npu.config.model_path) > 0) {
        int ret = npu_load_model(g_npu.config.model_path);
        if (ret < 0) {
            log_warn("Failed to load default model: %s", g_npu.config.model_path);
        }
    }

    log_info("NPU initialized successfully");
    return 0;
}

int npu_load_model(const char* model_path)
{
    int ret;
    FILE* fp;
    void* model_data = NULL;
    size_t model_size;

    if (!model_path) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_npu.mutex);

    if (!g_npu.status.initialized) {
        log_error("NPU not initialized");
        pthread_mutex_unlock(&g_npu.mutex);
        return -EINVAL;
    }

    // Unload previous model
    if (g_npu.ctx_valid) {
        rknn_destroy_stub(g_npu.ctx);
        g_npu.ctx_valid = false;
        g_npu.status.model_loaded = false;
    }

    log_info("Loading model: %s", model_path);

    // Read model file
    fp = fopen(model_path, "rb");
    if (!fp) {
        log_error("Failed to open model file: %s", strerror(errno));
        pthread_mutex_unlock(&g_npu.mutex);
        return -errno;
    }

    fseek(fp, 0, SEEK_END);
    model_size = ftell(fp);
    fseek(fp, 0, SEEK_SET);

    model_data = malloc(model_size);
    if (!model_data) {
        log_error("Failed to allocate memory for model");
        fclose(fp);
        pthread_mutex_unlock(&g_npu.mutex);
        return -ENOMEM;
    }

    if (fread(model_data, 1, model_size, fp) != model_size) {
        log_error("Failed to read model file");
        free(model_data);
        fclose(fp);
        pthread_mutex_unlock(&g_npu.mutex);
        return -EIO;
    }
    fclose(fp);

    // Initialize RKNN context
    ret = rknn_init_stub(&g_npu.ctx, model_data, model_size);
    free(model_data);

    if (ret < 0) {
        log_error("rknn_init failed: %d", ret);
        pthread_mutex_unlock(&g_npu.mutex);
        return ret;
    }

    g_npu.ctx_valid = true;
    g_npu.status.model_loaded = true;
    strncpy(g_npu.status.model_path, model_path, sizeof(g_npu.status.model_path) - 1);
    g_npu.status.num_outputs = 3;  // YOLOv8 has 3 output tensors

    pthread_mutex_unlock(&g_npu.mutex);

    log_info("Model loaded successfully: %zu bytes", model_size);
    return 0;
}

int npu_unload_model(void)
{
    pthread_mutex_lock(&g_npu.mutex);

    if (g_npu.ctx_valid) {
        rknn_destroy_stub(g_npu.ctx);
        g_npu.ctx_valid = false;
    }

    g_npu.status.model_loaded = false;
    memset(g_npu.status.model_path, 0, sizeof(g_npu.status.model_path));

    pthread_mutex_unlock(&g_npu.mutex);

    log_info("Model unloaded");
    return 0;
}

void npu_shutdown(void)
{
    pthread_mutex_lock(&g_npu.mutex);

    if (!g_npu.status.initialized) {
        pthread_mutex_unlock(&g_npu.mutex);
        return;
    }

    if (g_npu.ctx_valid) {
        rknn_destroy_stub(g_npu.ctx);
        g_npu.ctx_valid = false;
    }

    if (g_npu.input_buffer) {
        free(g_npu.input_buffer);
        g_npu.input_buffer = NULL;
    }

    if (g_npu.output_buffer) {
        free(g_npu.output_buffer);
        g_npu.output_buffer = NULL;
    }

    g_npu.status.initialized = false;
    g_npu.status.model_loaded = false;

    pthread_mutex_unlock(&g_npu.mutex);

    log_info("NPU shutdown complete");
}

const npu_status_t* npu_get_status(void)
{
    return &g_npu.status;
}

int npu_inference(const uint8_t* rgb_data, int width, int height,
                  npu_result_t* result)
{
    int ret;
    uint64_t start_us, end_us;

    if (!rgb_data || !result) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_npu.mutex);

    if (!g_npu.status.initialized || !g_npu.status.model_loaded) {
        log_error("NPU not ready for inference");
        pthread_mutex_unlock(&g_npu.mutex);
        return -EINVAL;
    }

    start_us = get_timestamp_us();

    // Resize to model input size if needed
    if (width != NPU_INPUT_WIDTH || height != NPU_INPUT_HEIGHT) {
        resize_rgb(rgb_data, width, height,
                   g_npu.input_buffer, NPU_INPUT_WIDTH, NPU_INPUT_HEIGHT);
    } else {
        memcpy(g_npu.input_buffer, rgb_data,
               NPU_INPUT_WIDTH * NPU_INPUT_HEIGHT * 3);
    }

    // Run inference
    ret = rknn_run_stub(g_npu.ctx);
    if (ret < 0) {
        log_error("rknn_run failed: %d", ret);
        pthread_mutex_unlock(&g_npu.mutex);
        return ret;
    }

    // Post-process YOLO output
    result->detection_count = yolo_postprocess(
        g_npu.output_buffer, 0,
        result->detections, NPU_MAX_DETECTIONS,
        g_npu.config.confidence_threshold,
        g_npu.config.nms_threshold
    );

    // Calculate angles for each detection
    for (int i = 0; i < result->detection_count; i++) {
        result->detections[i].azimuth_deg = isp_calculate_azimuth(
            result->detections[i].bbox_x,
            g_npu.config.horizontal_fov_deg
        );
        result->detections[i].elevation_deg = isp_calculate_elevation(
            result->detections[i].bbox_y,
            g_npu.config.vertical_fov_deg
        );
    }

    // Filter by class if person_only is set
    if (g_npu.config.person_only) {
        int write_idx = 0;
        for (int i = 0; i < result->detection_count; i++) {
            if (result->detections[i].class_id == YOLO_CLASS_PERSON) {
                if (write_idx != i) {
                    result->detections[write_idx] = result->detections[i];
                }
                write_idx++;
            }
        }
        result->detection_count = write_idx;
    }

    end_us = get_timestamp_us();
    int inference_time_ms = (int)((end_us - start_us) / 1000);

    result->timestamp_ns = get_timestamp_ns();
    result->frame_number = (uint32_t)g_npu.status.inference_count;
    result->inference_time_ms = inference_time_ms;

    // Update statistics
    g_npu.status.inference_count++;
    g_npu.total_inference_time_us += (end_us - start_us);
    g_npu.status.avg_inference_ms = (float)g_npu.total_inference_time_us /
                                     (float)g_npu.status.inference_count / 1000.0f;

    pthread_mutex_unlock(&g_npu.mutex);

    log_trace("Inference complete: %d detections in %dms",
              result->detection_count, inference_time_ms);

    return 0;
}

int npu_inference_nv12(const uint8_t* nv12_data, int width, int height,
                       npu_result_t* result)
{
    if (!nv12_data) {
        return -EINVAL;
    }

    // Allocate temporary RGB buffer
    size_t rgb_size = width * height * 3;
    uint8_t* rgb_buffer = (uint8_t*)malloc(rgb_size);
    if (!rgb_buffer) {
        return -ENOMEM;
    }

    // Convert NV12 to RGB
    nv12_to_rgb(nv12_data, width, height, rgb_buffer);

    // Run inference on RGB
    int ret = npu_inference(rgb_buffer, width, height, result);

    free(rgb_buffer);
    return ret;
}

int npu_set_confidence_threshold(float threshold)
{
    if (threshold < 0.0f || threshold > 1.0f) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_npu.mutex);
    g_npu.config.confidence_threshold = threshold;
    pthread_mutex_unlock(&g_npu.mutex);

    log_info("Confidence threshold set to %.2f", threshold);
    return 0;
}

int npu_set_nms_threshold(float threshold)
{
    if (threshold < 0.0f || threshold > 1.0f) {
        return -EINVAL;
    }

    pthread_mutex_lock(&g_npu.mutex);
    g_npu.config.nms_threshold = threshold;
    pthread_mutex_unlock(&g_npu.mutex);

    log_info("NMS threshold set to %.2f", threshold);
    return 0;
}

const char* npu_get_class_name(int32_t class_id)
{
    if (class_id < 0 || class_id >= NUM_COCO_CLASSES) {
        return "unknown";
    }
    return COCO_CLASSES[class_id];
}
