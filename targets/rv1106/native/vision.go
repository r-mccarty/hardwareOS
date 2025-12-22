// Package native provides Go bindings for the RS-1 vision pipeline.
// This file contains the vision pipeline types and high-level interfaces.
package native

import (
	"sync"
	"time"

	pb "github.com/jetkvm/kvm/targets/rv1106/native/proto"
)

// VisionDetection represents a single object detection from the NPU.
type VisionDetection struct {
	TrackID      int32   // Tracking ID (0 = unassigned)
	ClassID      int32   // Object class (COCO: 0=person, 2=car, etc.)
	Confidence   float32 // Detection confidence [0-1]
	BBoxX        float32 // Bounding box center X (normalized 0-1)
	BBoxY        float32 // Bounding box center Y (normalized 0-1)
	BBoxWidth    float32 // Bounding box width (normalized 0-1)
	BBoxHeight   float32 // Bounding box height (normalized 0-1)
	AzimuthDeg   float32 // Horizontal angle from camera center
	ElevationDeg float32 // Vertical angle from camera center
}

// VisionFrame represents a frame of detections from the NPU.
type VisionFrame struct {
	Detections    []VisionDetection
	TimestampNs   int64
	FrameNumber   int32
	InferenceMs   int32
}

// CameraState represents the camera subsystem status.
type CameraState struct {
	Initialized bool
	Streaming   bool
	Width       int32
	Height      int32
	FPS         int32
	FrameCount  uint64
	Error       string
}

// IspState represents the ISP subsystem status.
type IspState struct {
	Initialized    bool
	Running        bool
	LdchEnabled    bool
	LdchLevel      int32  // 0-255
	ExposureTimeMs float32
	Gain           float32
	ColorTempK     int32
}

// NpuState represents the NPU subsystem status.
type NpuState struct {
	Initialized    bool
	ModelLoaded    bool
	ModelPath      string
	InputWidth     int32
	InputHeight    int32
	InferenceCount uint64
	AvgInferenceMs float32
}

// VisionPipelineState represents the aggregate vision pipeline status.
type VisionPipelineState struct {
	Camera          CameraState
	Isp             IspState
	Npu             NpuState
	PipelineRunning bool
}

// VisionPipelineConfig holds configuration for the vision pipeline.
type VisionPipelineConfig struct {
	// Camera settings
	CameraWidth  uint32
	CameraHeight uint32
	CameraFPS    uint32

	// ISP settings
	LdchEnabled bool
	LdchLevel   uint8

	// NPU settings
	ModelPath            string
	ConfidenceThreshold  float32
	HorizontalFovDeg     float32
	VerticalFovDeg       float32
	PersonOnly           bool

	// Encoder settings
	EncodeWidth  uint32
	EncodeHeight uint32
	BitrateKbps  int32
	UseH265      bool
}

// VisionVideoCallback is called for each encoded video frame.
type VisionVideoCallback func(data []byte, timestampNs int64, isKeyframe bool)

// VisionDetectionCallback is called for each frame of NPU detections.
type VisionDetectionCallback func(frame *VisionFrame)

// VisionPipeline manages the camera → ISP → NPU pipeline.
type VisionPipeline struct {
	mu              sync.RWMutex
	config          VisionPipelineConfig
	running         bool
	videoCallback   VisionVideoCallback
	detectionCallback VisionDetectionCallback

	// Channels for async delivery
	visionFrameChan chan *VisionFrame
	videoFrameChan  chan visionVideoFrame

	// State
	state VisionPipelineState
}

type visionVideoFrame struct {
	data        []byte
	timestampNs int64
	isKeyframe  bool
}

// DefaultVisionPipelineConfig returns the default configuration.
func DefaultVisionPipelineConfig() VisionPipelineConfig {
	return VisionPipelineConfig{
		CameraWidth:         2304,
		CameraHeight:        1296,
		CameraFPS:           30,
		LdchEnabled:         true,
		LdchLevel:           255, // Full cylindrical projection
		ModelPath:           "/userdata/models/yolov8n.rknn",
		ConfidenceThreshold: 0.5,
		HorizontalFovDeg:    120.0,
		VerticalFovDeg:      90.0,
		PersonOnly:          false,
		EncodeWidth:         1920,
		EncodeHeight:        1080,
		BitrateKbps:         1500,
		UseH265:             true,
	}
}

// NewVisionPipeline creates a new vision pipeline instance.
func NewVisionPipeline(config *VisionPipelineConfig) *VisionPipeline {
	cfg := DefaultVisionPipelineConfig()
	if config != nil {
		cfg = *config
	}

	return &VisionPipeline{
		config:          cfg,
		visionFrameChan: make(chan *VisionFrame, 10),
		videoFrameChan:  make(chan visionVideoFrame, 10),
	}
}

// Initialize initializes the vision pipeline components.
func (vp *VisionPipeline) Initialize() error {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if err := visionPipelineInit(&vp.config); err != nil {
		return err
	}

	vp.updateState()
	return nil
}

// Shutdown shuts down the vision pipeline.
func (vp *VisionPipeline) Shutdown() {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if vp.running {
		vp.mu.Unlock()
		vp.Stop()
		vp.mu.Lock()
	}

	visionPipelineShutdown()
}

// Start starts the vision pipeline.
func (vp *VisionPipeline) Start(videoCb VisionVideoCallback, detectionCb VisionDetectionCallback) error {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	if vp.running {
		return nil
	}

	vp.videoCallback = videoCb
	vp.detectionCallback = detectionCb

	if err := visionPipelineStart(); err != nil {
		return err
	}

	vp.running = true

	// Start delivery goroutines
	go vp.handleVisionFrames()
	go vp.handleVideoFrames()

	return nil
}

// Stop stops the vision pipeline.
func (vp *VisionPipeline) Stop() error {
	vp.mu.Lock()
	if !vp.running {
		vp.mu.Unlock()
		return nil
	}
	vp.running = false
	vp.mu.Unlock()

	return visionPipelineStop()
}

// GetState returns the current pipeline state.
func (vp *VisionPipeline) GetState() VisionPipelineState {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	vp.updateState()
	return vp.state
}

// SetBitrate sets the video encoding bitrate.
func (vp *VisionPipeline) SetBitrate(bitrateKbps int32) error {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	vp.config.BitrateKbps = bitrateKbps
	return visionPipelineSetBitrate(bitrateKbps)
}

// RequestKeyframe requests an I-frame from the encoder.
func (vp *VisionPipeline) RequestKeyframe() error {
	return visionPipelineRequestKeyframe()
}

// SetNpuEnabled enables or disables NPU inference.
func (vp *VisionPipeline) SetNpuEnabled(enabled bool) error {
	return visionPipelineSetNpuEnabled(enabled)
}

// SetLdch configures LDCH lens distortion correction.
func (vp *VisionPipeline) SetLdch(enabled bool, level uint8) error {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	vp.config.LdchEnabled = enabled
	vp.config.LdchLevel = level
	return visionPipelineSetLdch(enabled, level)
}

// GetConfig returns the current configuration.
func (vp *VisionPipeline) GetConfig() VisionPipelineConfig {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	return vp.config
}

// IsRunning returns whether the pipeline is currently running.
func (vp *VisionPipeline) IsRunning() bool {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	return vp.running
}

func (vp *VisionPipeline) updateState() {
	status := visionPipelineGetStatus()
	if status == nil {
		return
	}

	vp.state = *status
}

func (vp *VisionPipeline) handleVisionFrames() {
	for vp.IsRunning() {
		select {
		case frame := <-vp.visionFrameChan:
			vp.mu.RLock()
			cb := vp.detectionCallback
			vp.mu.RUnlock()

			if cb != nil {
				cb(frame)
			}
		case <-time.After(100 * time.Millisecond):
			// Timeout to check running status
		}
	}
}

func (vp *VisionPipeline) handleVideoFrames() {
	for vp.IsRunning() {
		select {
		case frame := <-vp.videoFrameChan:
			vp.mu.RLock()
			cb := vp.videoCallback
			vp.mu.RUnlock()

			if cb != nil {
				cb(frame.data, frame.timestampNs, frame.isKeyframe)
			}
		case <-time.After(100 * time.Millisecond):
			// Timeout to check running status
		}
	}
}

// VisionFrameToProto converts a VisionFrame to its protobuf representation.
func VisionFrameToProto(frame *VisionFrame) *pb.VisionFrame {
	detections := make([]*pb.VisionDetection, len(frame.Detections))
	for i, d := range frame.Detections {
		detections[i] = &pb.VisionDetection{
			TrackId:      d.TrackID,
			BboxX:        d.BBoxX,
			BboxY:        d.BBoxY,
			BboxWidth:    d.BBoxWidth,
			BboxHeight:   d.BBoxHeight,
			ClassId:      d.ClassID,
			Confidence:   d.Confidence,
			AzimuthDeg:   d.AzimuthDeg,
			ElevationDeg: d.ElevationDeg,
		}
	}

	return &pb.VisionFrame{
		Detections:  detections,
		TimestampNs: frame.TimestampNs,
		FrameNumber: frame.FrameNumber,
		InferenceMs: frame.InferenceMs,
	}
}

// VisionPipelineStateToProto converts VisionPipelineState to protobuf.
func VisionPipelineStateToProto(state *VisionPipelineState) *pb.VisionPipelineState {
	return &pb.VisionPipelineState{
		Camera: &pb.CameraState{
			Initialized: state.Camera.Initialized,
			Streaming:   state.Camera.Streaming,
			Width:       state.Camera.Width,
			Height:      state.Camera.Height,
			Fps:         state.Camera.FPS,
			FrameCount:  state.Camera.FrameCount,
			Error:       state.Camera.Error,
		},
		Isp: &pb.IspState{
			Initialized:    state.Isp.Initialized,
			Running:        state.Isp.Running,
			LdchEnabled:    state.Isp.LdchEnabled,
			LdchLevel:      state.Isp.LdchLevel,
			ExposureTimeMs: state.Isp.ExposureTimeMs,
			Gain:           state.Isp.Gain,
			ColorTempK:     state.Isp.ColorTempK,
		},
		Npu: &pb.NpuState{
			Initialized:    state.Npu.Initialized,
			ModelLoaded:    state.Npu.ModelLoaded,
			ModelPath:      state.Npu.ModelPath,
			InputWidth:     state.Npu.InputWidth,
			InputHeight:    state.Npu.InputHeight,
			InferenceCount: state.Npu.InferenceCount,
			AvgInferenceMs: state.Npu.AvgInferenceMs,
		},
		PipelineRunning: state.PipelineRunning,
	}
}

// COCO class names for convenience
var COCOClassNames = map[int32]string{
	0:  "person",
	1:  "bicycle",
	2:  "car",
	3:  "motorcycle",
	4:  "airplane",
	5:  "bus",
	6:  "train",
	7:  "truck",
	8:  "boat",
	15: "cat",
	16: "dog",
	// Add more as needed
}

// GetClassName returns the COCO class name for a class ID.
func GetClassName(classID int32) string {
	if name, ok := COCOClassNames[classID]; ok {
		return name
	}
	return "unknown"
}
