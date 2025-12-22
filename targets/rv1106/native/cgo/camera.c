/**
 * @file camera.c
 * @brief SC3336 MIPI Camera Implementation for RS-1 Vision Pipeline
 *
 * This module initializes and controls the SC3336 3MP MIPI CSI-2 sensor
 * via V4L2 interface on the Rockchip RV1106G platform.
 */

#define _POSIX_C_SOURCE 200809L

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <pthread.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <linux/videodev2.h>
#include <time.h>

#include "camera.h"
#include "log.h"

// Buffer management
#define CAMERA_BUFFER_COUNT 4

// Buffer descriptor
typedef struct {
    void*   start;
    size_t  length;
    int     dma_fd;
} camera_buffer_t;

// Module state
static struct {
    int                     mipi_fd;
    int                     sensor_fd;
    camera_buffer_t         buffers[CAMERA_BUFFER_COUNT];
    int                     buffer_count;
    camera_config_t         config;
    camera_status_t         status;
    camera_frame_callback_t callback;
    void*                   user_data;
    pthread_t               capture_thread;
    volatile bool           running;
    pthread_mutex_t         mutex;
} g_camera = {
    .mipi_fd = -1,
    .sensor_fd = -1,
    .buffer_count = 0,
    .running = false,
    .mutex = PTHREAD_MUTEX_INITIALIZER,
};

static uint64_t get_timestamp_ns(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}

static int camera_set_sensor_format(void)
{
    struct v4l2_subdev_format fmt = {0};

    fmt.which = V4L2_SUBDEV_FORMAT_ACTIVE;
    fmt.pad = 0;
    fmt.format.width = g_camera.config.width;
    fmt.format.height = g_camera.config.height;
    fmt.format.code = 0x300f;  // MEDIA_BUS_FMT_SGRBG10_1X10 for SC3336
    fmt.format.field = V4L2_FIELD_NONE;

    if (ioctl(g_camera.sensor_fd, VIDIOC_SUBDEV_S_FMT, &fmt) < 0) {
        log_error("Failed to set sensor format: %s", strerror(errno));
        return -errno;
    }

    log_info("Sensor format set: %dx%d", fmt.format.width, fmt.format.height);
    return 0;
}

static int camera_set_mipi_format(void)
{
    struct v4l2_format fmt = {0};

    fmt.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    fmt.fmt.pix_mp.width = g_camera.config.width;
    fmt.fmt.pix_mp.height = g_camera.config.height;
    // Use NV12 (ISP output format after debayer + processing)
    fmt.fmt.pix_mp.pixelformat = V4L2_PIX_FMT_NV12;
    fmt.fmt.pix_mp.field = V4L2_FIELD_NONE;
    fmt.fmt.pix_mp.num_planes = 1;

    if (ioctl(g_camera.mipi_fd, VIDIOC_S_FMT, &fmt) < 0) {
        log_error("Failed to set MIPI format: %s", strerror(errno));
        return -errno;
    }

    // Set frame rate
    struct v4l2_streamparm parm = {0};
    parm.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    parm.parm.capture.timeperframe.numerator = 1;
    parm.parm.capture.timeperframe.denominator = g_camera.config.fps;

    if (ioctl(g_camera.mipi_fd, VIDIOC_S_PARM, &parm) < 0) {
        log_warn("Failed to set frame rate: %s (may be ignored)", strerror(errno));
    }

    log_info("MIPI format set: %dx%d @ %d fps, NV12",
             fmt.fmt.pix_mp.width, fmt.fmt.pix_mp.height, g_camera.config.fps);

    return 0;
}

static int camera_alloc_buffers(void)
{
    struct v4l2_requestbuffers req = {0};

    req.count = CAMERA_BUFFER_COUNT;
    req.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    req.memory = V4L2_MEMORY_MMAP;

    if (ioctl(g_camera.mipi_fd, VIDIOC_REQBUFS, &req) < 0) {
        log_error("Failed to request buffers: %s", strerror(errno));
        return -errno;
    }

    if (req.count < 2) {
        log_error("Insufficient buffer memory");
        return -ENOMEM;
    }

    g_camera.buffer_count = req.count;
    log_info("Allocated %d capture buffers", req.count);

    // Map buffers
    for (int i = 0; i < g_camera.buffer_count; i++) {
        struct v4l2_buffer buf = {0};
        struct v4l2_plane planes[1] = {0};

        buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
        buf.memory = V4L2_MEMORY_MMAP;
        buf.index = i;
        buf.length = 1;
        buf.m.planes = planes;

        if (ioctl(g_camera.mipi_fd, VIDIOC_QUERYBUF, &buf) < 0) {
            log_error("Failed to query buffer %d: %s", i, strerror(errno));
            return -errno;
        }

        g_camera.buffers[i].length = planes[0].length;
        g_camera.buffers[i].start = mmap(
            NULL,
            planes[0].length,
            PROT_READ | PROT_WRITE,
            MAP_SHARED,
            g_camera.mipi_fd,
            planes[0].m.mem_offset
        );

        if (g_camera.buffers[i].start == MAP_FAILED) {
            log_error("Failed to mmap buffer %d: %s", i, strerror(errno));
            return -errno;
        }

        log_trace("Buffer %d mapped: %zu bytes at %p", i,
                  g_camera.buffers[i].length, g_camera.buffers[i].start);
    }

    return 0;
}

static void camera_free_buffers(void)
{
    for (int i = 0; i < g_camera.buffer_count; i++) {
        if (g_camera.buffers[i].start && g_camera.buffers[i].start != MAP_FAILED) {
            munmap(g_camera.buffers[i].start, g_camera.buffers[i].length);
            g_camera.buffers[i].start = NULL;
        }
    }
    g_camera.buffer_count = 0;
}

static int camera_queue_buffers(void)
{
    for (int i = 0; i < g_camera.buffer_count; i++) {
        struct v4l2_buffer buf = {0};
        struct v4l2_plane planes[1] = {0};

        buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
        buf.memory = V4L2_MEMORY_MMAP;
        buf.index = i;
        buf.length = 1;
        buf.m.planes = planes;

        if (ioctl(g_camera.mipi_fd, VIDIOC_QBUF, &buf) < 0) {
            log_error("Failed to queue buffer %d: %s", i, strerror(errno));
            return -errno;
        }
    }

    return 0;
}

static void* camera_capture_thread(void* arg)
{
    (void)arg;

    struct v4l2_buffer buf = {0};
    struct v4l2_plane planes[1] = {0};
    fd_set fds;
    struct timeval tv;
    camera_frame_t frame;

    log_info("Camera capture thread started");

    while (g_camera.running) {
        FD_ZERO(&fds);
        FD_SET(g_camera.mipi_fd, &fds);

        tv.tv_sec = 1;
        tv.tv_usec = 0;

        int ret = select(g_camera.mipi_fd + 1, &fds, NULL, NULL, &tv);

        if (ret == 0) {
            log_warn("Camera select timeout");
            continue;
        }

        if (ret < 0) {
            if (errno == EINTR) {
                continue;
            }
            log_error("Camera select error: %s", strerror(errno));
            break;
        }

        // Dequeue buffer
        memset(&buf, 0, sizeof(buf));
        memset(planes, 0, sizeof(planes));
        buf.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
        buf.memory = V4L2_MEMORY_MMAP;
        buf.length = 1;
        buf.m.planes = planes;

        if (ioctl(g_camera.mipi_fd, VIDIOC_DQBUF, &buf) < 0) {
            if (errno == EAGAIN) {
                continue;
            }
            log_error("Failed to dequeue buffer: %s", strerror(errno));
            break;
        }

        // Build frame descriptor
        frame.data = g_camera.buffers[buf.index].start;
        frame.size = planes[0].bytesused;
        frame.width = g_camera.config.width;
        frame.height = g_camera.config.height;
        frame.dma_fd = -1;  // MMAP mode, no DMA fd
        frame.timestamp_ns = get_timestamp_ns();
        frame.sequence = buf.sequence;

        // Update status
        pthread_mutex_lock(&g_camera.mutex);
        g_camera.status.frame_count++;
        g_camera.status.last_frame_ts_ns = frame.timestamp_ns;
        pthread_mutex_unlock(&g_camera.mutex);

        // Deliver frame via callback
        if (g_camera.callback) {
            g_camera.callback(&frame, g_camera.user_data);
        }

        // Re-queue buffer
        if (ioctl(g_camera.mipi_fd, VIDIOC_QBUF, &buf) < 0) {
            log_error("Failed to re-queue buffer: %s", strerror(errno));
            break;
        }
    }

    log_info("Camera capture thread exiting");
    return NULL;
}

int camera_init(const camera_config_t* config)
{
    int ret;

    pthread_mutex_lock(&g_camera.mutex);

    if (g_camera.status.initialized) {
        log_warn("Camera already initialized");
        pthread_mutex_unlock(&g_camera.mutex);
        return 0;
    }

    // Apply configuration
    if (config) {
        g_camera.config = *config;
    } else {
        // Default configuration for SC3336
        g_camera.config.width = CAMERA_SENSOR_WIDTH;
        g_camera.config.height = CAMERA_SENSOR_HEIGHT;
        g_camera.config.fps = CAMERA_SENSOR_FPS;
        g_camera.config.hdr_enabled = false;
        g_camera.config.mirror = false;
        g_camera.config.flip = false;
    }

    log_info("Initializing SC3336 camera: %dx%d @ %d fps",
             g_camera.config.width, g_camera.config.height, g_camera.config.fps);

    // Open sensor subdev for control
    g_camera.sensor_fd = open(CAMERA_SENSOR_DEV, O_RDWR);
    if (g_camera.sensor_fd < 0) {
        log_warn("Failed to open sensor device %s: %s (may not exist on this platform)",
                 CAMERA_SENSOR_DEV, strerror(errno));
        // Continue anyway - some platforms expose everything through video node
    }

    // Open MIPI video device
    g_camera.mipi_fd = open(CAMERA_MIPI_DEV, O_RDWR | O_NONBLOCK);
    if (g_camera.mipi_fd < 0) {
        log_error("Failed to open camera device %s: %s",
                  CAMERA_MIPI_DEV, strerror(errno));
        ret = -errno;
        goto error;
    }

    // Configure sensor format
    if (g_camera.sensor_fd >= 0) {
        ret = camera_set_sensor_format();
        if (ret < 0) {
            log_warn("Sensor format configuration failed (continuing)");
        }
    }

    // Configure MIPI capture format
    ret = camera_set_mipi_format();
    if (ret < 0) {
        goto error;
    }

    // Allocate capture buffers
    ret = camera_alloc_buffers();
    if (ret < 0) {
        goto error;
    }

    // Update status
    g_camera.status.initialized = true;
    g_camera.status.streaming = false;
    g_camera.status.width = g_camera.config.width;
    g_camera.status.height = g_camera.config.height;
    g_camera.status.fps = g_camera.config.fps;
    g_camera.status.frame_count = 0;

    pthread_mutex_unlock(&g_camera.mutex);

    log_info("SC3336 camera initialized successfully");
    return 0;

error:
    if (g_camera.mipi_fd >= 0) {
        close(g_camera.mipi_fd);
        g_camera.mipi_fd = -1;
    }
    if (g_camera.sensor_fd >= 0) {
        close(g_camera.sensor_fd);
        g_camera.sensor_fd = -1;
    }
    pthread_mutex_unlock(&g_camera.mutex);
    return ret;
}

void camera_shutdown(void)
{
    pthread_mutex_lock(&g_camera.mutex);

    if (!g_camera.status.initialized) {
        pthread_mutex_unlock(&g_camera.mutex);
        return;
    }

    // Stop streaming if running
    if (g_camera.status.streaming) {
        pthread_mutex_unlock(&g_camera.mutex);
        camera_stop_streaming();
        pthread_mutex_lock(&g_camera.mutex);
    }

    // Free buffers
    camera_free_buffers();

    // Close devices
    if (g_camera.mipi_fd >= 0) {
        close(g_camera.mipi_fd);
        g_camera.mipi_fd = -1;
    }
    if (g_camera.sensor_fd >= 0) {
        close(g_camera.sensor_fd);
        g_camera.sensor_fd = -1;
    }

    g_camera.status.initialized = false;

    pthread_mutex_unlock(&g_camera.mutex);

    log_info("Camera shutdown complete");
}

int camera_start_streaming(camera_frame_callback_t callback, void* user_data)
{
    int ret;
    enum v4l2_buf_type type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;

    pthread_mutex_lock(&g_camera.mutex);

    if (!g_camera.status.initialized) {
        log_error("Camera not initialized");
        pthread_mutex_unlock(&g_camera.mutex);
        return -EINVAL;
    }

    if (g_camera.status.streaming) {
        log_warn("Camera already streaming");
        pthread_mutex_unlock(&g_camera.mutex);
        return 0;
    }

    g_camera.callback = callback;
    g_camera.user_data = user_data;

    // Queue all buffers
    ret = camera_queue_buffers();
    if (ret < 0) {
        pthread_mutex_unlock(&g_camera.mutex);
        return ret;
    }

    // Start streaming
    if (ioctl(g_camera.mipi_fd, VIDIOC_STREAMON, &type) < 0) {
        log_error("Failed to start streaming: %s", strerror(errno));
        pthread_mutex_unlock(&g_camera.mutex);
        return -errno;
    }

    // Start capture thread
    g_camera.running = true;
    if (pthread_create(&g_camera.capture_thread, NULL, camera_capture_thread, NULL) != 0) {
        log_error("Failed to create capture thread");
        ioctl(g_camera.mipi_fd, VIDIOC_STREAMOFF, &type);
        g_camera.running = false;
        pthread_mutex_unlock(&g_camera.mutex);
        return -ENOMEM;
    }

    g_camera.status.streaming = true;
    g_camera.status.frame_count = 0;

    pthread_mutex_unlock(&g_camera.mutex);

    log_info("Camera streaming started");
    return 0;
}

int camera_stop_streaming(void)
{
    enum v4l2_buf_type type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;

    pthread_mutex_lock(&g_camera.mutex);

    if (!g_camera.status.streaming) {
        pthread_mutex_unlock(&g_camera.mutex);
        return 0;
    }

    // Signal thread to stop
    g_camera.running = false;

    pthread_mutex_unlock(&g_camera.mutex);

    // Wait for thread
    pthread_join(g_camera.capture_thread, NULL);

    pthread_mutex_lock(&g_camera.mutex);

    // Stop streaming
    if (ioctl(g_camera.mipi_fd, VIDIOC_STREAMOFF, &type) < 0) {
        log_warn("Failed to stop streaming: %s", strerror(errno));
    }

    g_camera.status.streaming = false;
    g_camera.callback = NULL;
    g_camera.user_data = NULL;

    pthread_mutex_unlock(&g_camera.mutex);

    log_info("Camera streaming stopped (frames captured: %lu)",
             (unsigned long)g_camera.status.frame_count);

    return 0;
}

const camera_status_t* camera_get_status(void)
{
    return &g_camera.status;
}

int camera_set_exposure(bool auto_exposure, int32_t manual_value)
{
    struct v4l2_control ctrl = {0};

    if (g_camera.mipi_fd < 0) {
        return -EINVAL;
    }

    // Set auto exposure
    ctrl.id = V4L2_CID_EXPOSURE_AUTO;
    ctrl.value = auto_exposure ? V4L2_EXPOSURE_AUTO : V4L2_EXPOSURE_MANUAL;

    if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
        log_warn("Failed to set exposure mode: %s", strerror(errno));
        return -errno;
    }

    if (!auto_exposure) {
        ctrl.id = V4L2_CID_EXPOSURE_ABSOLUTE;
        ctrl.value = manual_value;

        if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
            log_warn("Failed to set exposure value: %s", strerror(errno));
            return -errno;
        }
    }

    log_info("Exposure set: auto=%d, value=%d", auto_exposure, manual_value);
    return 0;
}

int camera_set_gain(bool auto_gain, int32_t manual_value)
{
    struct v4l2_control ctrl = {0};

    if (g_camera.mipi_fd < 0) {
        return -EINVAL;
    }

    // Set auto gain
    ctrl.id = V4L2_CID_AUTOGAIN;
    ctrl.value = auto_gain ? 1 : 0;

    if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
        log_warn("Failed to set gain mode: %s", strerror(errno));
        return -errno;
    }

    if (!auto_gain) {
        ctrl.id = V4L2_CID_GAIN;
        ctrl.value = manual_value;

        if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
            log_warn("Failed to set gain value: %s", strerror(errno));
            return -errno;
        }
    }

    log_info("Gain set: auto=%d, value=%d", auto_gain, manual_value);
    return 0;
}

int camera_set_orientation(bool mirror, bool flip)
{
    struct v4l2_control ctrl = {0};
    int ret = 0;

    if (g_camera.mipi_fd < 0) {
        return -EINVAL;
    }

    ctrl.id = V4L2_CID_HFLIP;
    ctrl.value = mirror ? 1 : 0;
    if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
        log_warn("Failed to set horizontal flip: %s", strerror(errno));
        ret = -errno;
    }

    ctrl.id = V4L2_CID_VFLIP;
    ctrl.value = flip ? 1 : 0;
    if (ioctl(g_camera.mipi_fd, VIDIOC_S_CTRL, &ctrl) < 0) {
        log_warn("Failed to set vertical flip: %s", strerror(errno));
        ret = -errno;
    }

    g_camera.config.mirror = mirror;
    g_camera.config.flip = flip;

    log_info("Orientation set: mirror=%d, flip=%d", mirror, flip);
    return ret;
}
