//go:build linux

package native

/*
#cgo LDFLAGS: -Lcgo/lib -ljknative -llvgl
#cgo CFLAGS: -Icgo/include
#include "vision_pipeline.h"
#include <stdlib.h>
#include <string.h>

// Callback wrappers - defined as extern in Go
extern void jetkvm_go_vision_video_callback(const uint8_t* data, size_t size, uint64_t timestamp_ns, int is_keyframe);
extern void jetkvm_go_vision_detection_callback(int detection_count, int32_t* track_ids, int32_t* class_ids,
    float* confidences, float* bbox_x, float* bbox_y, float* bbox_width, float* bbox_height,
    float* azimuth_deg, float* elevation_deg, int64_t timestamp_ns, int32_t frame_number, int32_t inference_ms);

// C callback wrappers that match the C function signatures
static void c_vision_video_callback(const uint8_t* data, size_t size, uint64_t timestamp_ns, bool is_keyframe, void* user_data) {
    (void)user_data;
    jetkvm_go_vision_video_callback(data, size, timestamp_ns, is_keyframe ? 1 : 0);
}

static void c_vision_detection_callback(const npu_result_t* result, void* user_data) {
    (void)user_data;
    if (result == NULL || result->detection_count == 0) {
        return;
    }

    int count = result->detection_count;
    if (count > NPU_MAX_DETECTIONS) {
        count = NPU_MAX_DETECTIONS;
    }

    // Allocate arrays for batch transfer to Go
    int32_t* track_ids = (int32_t*)malloc(count * sizeof(int32_t));
    int32_t* class_ids = (int32_t*)malloc(count * sizeof(int32_t));
    float* confidences = (float*)malloc(count * sizeof(float));
    float* bbox_x = (float*)malloc(count * sizeof(float));
    float* bbox_y = (float*)malloc(count * sizeof(float));
    float* bbox_width = (float*)malloc(count * sizeof(float));
    float* bbox_height = (float*)malloc(count * sizeof(float));
    float* azimuth_deg = (float*)malloc(count * sizeof(float));
    float* elevation_deg = (float*)malloc(count * sizeof(float));

    for (int i = 0; i < count; i++) {
        track_ids[i] = result->detections[i].track_id;
        class_ids[i] = result->detections[i].class_id;
        confidences[i] = result->detections[i].confidence;
        bbox_x[i] = result->detections[i].bbox_x;
        bbox_y[i] = result->detections[i].bbox_y;
        bbox_width[i] = result->detections[i].bbox_width;
        bbox_height[i] = result->detections[i].bbox_height;
        azimuth_deg[i] = result->detections[i].azimuth_deg;
        elevation_deg[i] = result->detections[i].elevation_deg;
    }

    jetkvm_go_vision_detection_callback(count, track_ids, class_ids, confidences,
        bbox_x, bbox_y, bbox_width, bbox_height, azimuth_deg, elevation_deg,
        result->timestamp_ns, result->frame_number, result->inference_time_ms);

    free(track_ids);
    free(class_ids);
    free(confidences);
    free(bbox_x);
    free(bbox_y);
    free(bbox_width);
    free(bbox_height);
    free(azimuth_deg);
    free(elevation_deg);
}

static inline int cgo_vision_pipeline_init(
    uint32_t camera_width, uint32_t camera_height, uint32_t camera_fps,
    int ldch_enabled, uint8_t ldch_level,
    const char* model_path, float confidence_threshold,
    float horizontal_fov_deg, float vertical_fov_deg, int person_only,
    uint32_t encode_width, uint32_t encode_height, int32_t bitrate_kbps, int use_h265)
{
    vision_pipeline_config_t config;
    config.camera_width = camera_width;
    config.camera_height = camera_height;
    config.camera_fps = camera_fps;
    config.ldch_enabled = ldch_enabled != 0;
    config.ldch_level = ldch_level;
    strncpy(config.model_path, model_path, sizeof(config.model_path) - 1);
    config.model_path[sizeof(config.model_path) - 1] = '\0';
    config.confidence_threshold = confidence_threshold;
    config.horizontal_fov_deg = horizontal_fov_deg;
    config.vertical_fov_deg = vertical_fov_deg;
    config.person_only = person_only != 0;
    config.encode_width = encode_width;
    config.encode_height = encode_height;
    config.bitrate_kbps = bitrate_kbps;
    config.use_h265 = use_h265 != 0;

    return vision_pipeline_init(&config);
}

static inline int cgo_vision_pipeline_start() {
    return vision_pipeline_start(c_vision_video_callback, c_vision_detection_callback, NULL);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

var (
	// Global channels for vision callbacks
	visionVideoFrameChan     chan visionVideoFrame
	visionDetectionFrameChan chan *VisionFrame
)

func init() {
	visionVideoFrameChan = make(chan visionVideoFrame, 10)
	visionDetectionFrameChan = make(chan *VisionFrame, 10)
}

//export jetkvm_go_vision_video_callback
func jetkvm_go_vision_video_callback(data *C.uint8_t, size C.size_t, timestampNs C.uint64_t, isKeyframe C.int) {
	frame := visionVideoFrame{
		data:        C.GoBytes(unsafe.Pointer(data), C.int(size)),
		timestampNs: int64(timestampNs),
		isKeyframe:  isKeyframe != 0,
	}

	select {
	case visionVideoFrameChan <- frame:
	default:
		// Drop frame if channel is full
	}
}

//export jetkvm_go_vision_detection_callback
func jetkvm_go_vision_detection_callback(
	detectionCount C.int,
	trackIDs *C.int32_t,
	classIDs *C.int32_t,
	confidences *C.float,
	bboxX *C.float,
	bboxY *C.float,
	bboxWidth *C.float,
	bboxHeight *C.float,
	azimuthDeg *C.float,
	elevationDeg *C.float,
	timestampNs C.int64_t,
	frameNumber C.int32_t,
	inferenceMs C.int32_t,
) {
	count := int(detectionCount)
	if count <= 0 {
		return
	}

	// Convert C arrays to Go slices
	trackIDSlice := (*[100]C.int32_t)(unsafe.Pointer(trackIDs))[:count:count]
	classIDSlice := (*[100]C.int32_t)(unsafe.Pointer(classIDs))[:count:count]
	confSlice := (*[100]C.float)(unsafe.Pointer(confidences))[:count:count]
	bboxXSlice := (*[100]C.float)(unsafe.Pointer(bboxX))[:count:count]
	bboxYSlice := (*[100]C.float)(unsafe.Pointer(bboxY))[:count:count]
	bboxWSlice := (*[100]C.float)(unsafe.Pointer(bboxWidth))[:count:count]
	bboxHSlice := (*[100]C.float)(unsafe.Pointer(bboxHeight))[:count:count]
	azimuthSlice := (*[100]C.float)(unsafe.Pointer(azimuthDeg))[:count:count]
	elevationSlice := (*[100]C.float)(unsafe.Pointer(elevationDeg))[:count:count]

	detections := make([]VisionDetection, count)
	for i := 0; i < count; i++ {
		detections[i] = VisionDetection{
			TrackID:      int32(trackIDSlice[i]),
			ClassID:      int32(classIDSlice[i]),
			Confidence:   float32(confSlice[i]),
			BBoxX:        float32(bboxXSlice[i]),
			BBoxY:        float32(bboxYSlice[i]),
			BBoxWidth:    float32(bboxWSlice[i]),
			BBoxHeight:   float32(bboxHSlice[i]),
			AzimuthDeg:   float32(azimuthSlice[i]),
			ElevationDeg: float32(elevationSlice[i]),
		}
	}

	frame := &VisionFrame{
		Detections:  detections,
		TimestampNs: int64(timestampNs),
		FrameNumber: int32(frameNumber),
		InferenceMs: int32(inferenceMs),
	}

	select {
	case visionDetectionFrameChan <- frame:
	default:
		// Drop frame if channel is full
	}
}

func visionPipelineInit(config *VisionPipelineConfig) error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	modelPath := C.CString(config.ModelPath)
	defer C.free(unsafe.Pointer(modelPath))

	ldchEnabled := 0
	if config.LdchEnabled {
		ldchEnabled = 1
	}
	personOnly := 0
	if config.PersonOnly {
		personOnly = 1
	}
	useH265 := 0
	if config.UseH265 {
		useH265 = 1
	}

	ret := C.cgo_vision_pipeline_init(
		C.uint32_t(config.CameraWidth),
		C.uint32_t(config.CameraHeight),
		C.uint32_t(config.CameraFPS),
		C.int(ldchEnabled),
		C.uint8_t(config.LdchLevel),
		modelPath,
		C.float(config.ConfidenceThreshold),
		C.float(config.HorizontalFovDeg),
		C.float(config.VerticalFovDeg),
		C.int(personOnly),
		C.uint32_t(config.EncodeWidth),
		C.uint32_t(config.EncodeHeight),
		C.int32_t(config.BitrateKbps),
		C.int(useH265),
	)

	if ret != 0 {
		return fmt.Errorf("vision_pipeline_init failed: %d", ret)
	}
	return nil
}

func visionPipelineShutdown() {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	C.vision_pipeline_shutdown()
}

func visionPipelineStart() error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.cgo_vision_pipeline_start()
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_start failed: %d", ret)
	}
	return nil
}

func visionPipelineStop() error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.vision_pipeline_stop()
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_stop failed: %d", ret)
	}
	return nil
}

func visionPipelineGetStatus() *VisionPipelineState {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	status := C.vision_pipeline_get_status()
	if status == nil {
		return nil
	}

	return &VisionPipelineState{
		Camera: CameraState{
			Initialized: bool(status.camera_ready),
			Streaming:   bool(status.running),
			FrameCount:  uint64(status.frames_captured),
		},
		Isp: IspState{
			Initialized: bool(status.isp_ready),
			Running:     bool(status.running),
		},
		Npu: NpuState{
			Initialized:    bool(status.npu_ready),
			InferenceCount: uint64(status.frames_inferred),
			AvgInferenceMs: float32(status.avg_inference_ms),
		},
		PipelineRunning: bool(status.running),
	}
}

func visionPipelineSetBitrate(bitrateKbps int32) error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.vision_pipeline_set_bitrate(C.int32_t(bitrateKbps))
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_set_bitrate failed: %d", ret)
	}
	return nil
}

func visionPipelineRequestKeyframe() error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.vision_pipeline_request_keyframe()
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_request_keyframe failed: %d", ret)
	}
	return nil
}

func visionPipelineSetNpuEnabled(enabled bool) error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.vision_pipeline_set_npu_enabled(C.bool(enabled))
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_set_npu_enabled failed: %d", ret)
	}
	return nil
}

func visionPipelineSetLdch(enabled bool, level uint8) error {
	cgoLock.Lock()
	defer cgoLock.Unlock()

	ret := C.vision_pipeline_set_ldch(C.bool(enabled), C.uint8_t(level))
	if ret != 0 {
		return fmt.Errorf("vision_pipeline_set_ldch failed: %d", ret)
	}
	return nil
}

// GetVisionVideoFrameChan returns the channel for vision video frames.
func GetVisionVideoFrameChan() <-chan visionVideoFrame {
	return visionVideoFrameChan
}

// GetVisionDetectionFrameChan returns the channel for vision detection frames.
func GetVisionDetectionFrameChan() <-chan *VisionFrame {
	return visionDetectionFrameChan
}
