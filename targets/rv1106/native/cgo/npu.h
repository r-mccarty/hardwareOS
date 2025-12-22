/**
 * @file npu.h
 * @brief Rockchip NPU Interface for RS-1 Vision Pipeline
 *
 * Runs YOLOv8 object detection inference on the Rockchip RKNN NPU.
 * RV1106G NPU: 0.5 TOPS
 * Model: YOLOv8n (nano) converted to .rknn format
 */

#ifndef RS1_NPU_H
#define RS1_NPU_H

#include <stdbool.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// NPU configuration
#define NPU_INPUT_WIDTH     640
#define NPU_INPUT_HEIGHT    640
#define NPU_INPUT_CHANNELS  3
#define NPU_MAX_DETECTIONS  100

// Default model path
#define NPU_DEFAULT_MODEL_PATH  "/userdata/models/yolov8n.rknn"

// YOLO class IDs (COCO dataset)
#define YOLO_CLASS_PERSON   0
#define YOLO_CLASS_CAR      2
#define YOLO_CLASS_CAT      15
#define YOLO_CLASS_DOG      16

// Detection result
typedef struct {
    int32_t  track_id;      // Tracking ID (0 = unassigned)
    int32_t  class_id;      // Object class (COCO)
    float    confidence;    // Detection confidence [0-1]
    float    bbox_x;        // Bounding box center X (normalized 0-1)
    float    bbox_y;        // Bounding box center Y (normalized 0-1)
    float    bbox_width;    // Bounding box width (normalized 0-1)
    float    bbox_height;   // Bounding box height (normalized 0-1)
    float    azimuth_deg;   // Horizontal angle from camera center
    float    elevation_deg; // Vertical angle from camera center
} npu_detection_t;

// Inference result
typedef struct {
    npu_detection_t detections[NPU_MAX_DETECTIONS];
    int32_t         detection_count;
    uint64_t        timestamp_ns;
    uint32_t        frame_number;
    int32_t         inference_time_ms;
} npu_result_t;

// NPU configuration
typedef struct {
    char     model_path[256];
    float    confidence_threshold;  // Min confidence to report (default: 0.5)
    float    nms_threshold;         // NMS IoU threshold (default: 0.45)
    float    horizontal_fov_deg;    // Camera horizontal FOV for azimuth calc
    float    vertical_fov_deg;      // Camera vertical FOV for elevation calc
    bool     person_only;           // Only detect class 0 (person)
} npu_config_t;

// NPU status
typedef struct {
    bool        initialized;
    bool        model_loaded;
    char        model_path[256];
    int32_t     input_width;
    int32_t     input_height;
    int32_t     num_outputs;
    uint64_t    inference_count;
    float       avg_inference_ms;
} npu_status_t;

// Callback for inference results
typedef void (*npu_result_callback_t)(const npu_result_t* result, void* user_data);

/**
 * @brief Initialize the NPU
 * @param config NPU configuration (NULL for defaults)
 * @return 0 on success, negative error code on failure
 */
int npu_init(const npu_config_t* config);

/**
 * @brief Load RKNN model
 * @param model_path Path to .rknn model file
 * @return 0 on success, negative error code on failure
 */
int npu_load_model(const char* model_path);

/**
 * @brief Unload current model
 * @return 0 on success, negative error code on failure
 */
int npu_unload_model(void);

/**
 * @brief Shutdown NPU subsystem
 */
void npu_shutdown(void);

/**
 * @brief Get current NPU status
 * @return Pointer to NPU status structure
 */
const npu_status_t* npu_get_status(void);

/**
 * @brief Run inference on RGB image
 * @param rgb_data RGB24 image data (640x640x3)
 * @param width Image width (must be NPU_INPUT_WIDTH)
 * @param height Image height (must be NPU_INPUT_HEIGHT)
 * @param result Output inference result
 * @return 0 on success, negative error code on failure
 */
int npu_inference(const uint8_t* rgb_data, int width, int height,
                  npu_result_t* result);

/**
 * @brief Run inference on NV12 image (with internal conversion)
 * @param nv12_data NV12 image data
 * @param width Image width
 * @param height Image height
 * @param result Output inference result
 * @return 0 on success, negative error code on failure
 */
int npu_inference_nv12(const uint8_t* nv12_data, int width, int height,
                       npu_result_t* result);

/**
 * @brief Set detection confidence threshold
 * @param threshold Minimum confidence [0-1]
 * @return 0 on success, negative error code on failure
 */
int npu_set_confidence_threshold(float threshold);

/**
 * @brief Set NMS IoU threshold
 * @param threshold NMS threshold [0-1]
 * @return 0 on success, negative error code on failure
 */
int npu_set_nms_threshold(float threshold);

/**
 * @brief Get class name for class ID
 * @param class_id COCO class ID
 * @return Class name string (static)
 */
const char* npu_get_class_name(int32_t class_id);

#ifdef __cplusplus
}
#endif

#endif // RS1_NPU_H
