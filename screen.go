// main.go
package main

/*
#cgo pkg-config: libavcodec libavutil libswscale libswresample
#cgo CFLAGS: -IE:\MYT\mytrpa\include
#cgo LDFLAGS: -L${SRCDIR} -lmytrpc
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include <sys/time.h>
#include "libmytrpc.h"

//static void (*onVideoStream)(int rot,void* data,int len);
//static void(*onAudioStream)(void* data, int len);

// 声明回调函数，由 Go 实现
extern void videoStreamCallback(int rot, void* data, int len);
// extern void audioStreamCallback(void* data, int len);

extern void addaudio();

#include <libavcodec/avcodec.h>
#include <libavutil/imgutils.h>
#include <libswscale/swscale.h>
#include <libavutil/channel_layout.h>
#include <pthread.h>
typedef struct {
    AVCodecContext *codec_ctx;
    AVFrame *frame;
    struct SwsContext *sws_ctx;
    uint8_t *buffer;
    int width;
    int height;
    int sws_src_format;
    AVBufferRef *hw_device_ctx;
} Decoder;

// 硬件解码器类型优先级配置
static const enum AVHWDeviceType hw_priority[] = {
    AV_HWDEVICE_TYPE_D3D12VA,
    AV_HWDEVICE_TYPE_D3D11VA,    // Windows Direct3D 11
    AV_HWDEVICE_TYPE_DXVA2,      // Windows DirectX Video Acceleration
    AV_HWDEVICE_TYPE_CUDA,       // NVIDIA CUDA
    AV_HWDEVICE_TYPE_QSV,        // Intel Quick Sync
    AV_HWDEVICE_TYPE_AMF,        // AMD AMF
    AV_HWDEVICE_TYPE_VULKAN,
    AV_HWDEVICE_TYPE_VAAPI,
    AV_HWDEVICE_TYPE_VDPAU,
    AV_HWDEVICE_TYPE_NONE        // 软件回退
};

// 添加音频队列相关定义
#define AUDIO_QUEUE_SIZE 50
typedef struct {
    uint8_t *data;
    int size;
} AudioPacket;

static AudioPacket audio_packet_queue[AUDIO_QUEUE_SIZE];
static int audio_queue_head = 0;
static int audio_queue_tail = 0;
static pthread_mutex_t audio_queue_mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t audio_queue_cond = PTHREAD_COND_INITIALIZER;
static volatile int audio_processing = 0;


static Decoder* create_decoder() {
    // 支持的硬件解码器优先级列表
    const char *hw_decoders[] = {
        "h264_d3d12va", // D3D11 Video Acceleration
        "h264_d3d11va", // D3D11 Video Acceleration
        //"h264_cuvid",   // NVIDIA CUVID
        //"h264_qsv",     // Intel QSV
        //"h264_amf",     // AMD AMF
        NULL            // 最后尝试软解
    };

    const AVCodec *codec = NULL;
    AVBufferRef *hw_device_ctx = NULL;
    AVCodecContext *codec_ctx = NULL;
    Decoder *d = NULL;

    // 尝试硬件解码器
    for (int i = 0; hw_decoders[i]; i++) {
        codec = avcodec_find_decoder_by_name(hw_decoders[i]);
        if (!codec) continue;

        enum AVHWDeviceType type = AV_HWDEVICE_TYPE_NONE;
        if (strstr(hw_decoders[i], "cuvid")) type = AV_HWDEVICE_TYPE_CUDA;
        else if (strstr(hw_decoders[i], "qsv")) type = AV_HWDEVICE_TYPE_QSV;
        else if (strstr(hw_decoders[i], "d3d11va")) type = AV_HWDEVICE_TYPE_D3D11VA;
        else if (strstr(hw_decoders[i], "d3d12va")) type = AV_HWDEVICE_TYPE_D3D12VA;
        else if (strstr(hw_decoders[i], "amf")) type = AV_HWDEVICE_TYPE_D3D11VA;

        if (av_hwdevice_ctx_create(&hw_device_ctx, type, NULL, NULL, 0) < 0) {
            hw_device_ctx = NULL;
            continue;
        }

        codec_ctx = avcodec_alloc_context3(codec);
        if (!codec_ctx) continue;

		codec_ctx->pkt_timebase = (AVRational){1, 1000};

        codec_ctx->hw_device_ctx = av_buffer_ref(hw_device_ctx);
        if (avcodec_open2(codec_ctx, codec, NULL) == 0) break;

        avcodec_free_context(&codec_ctx);
        av_buffer_unref(&hw_device_ctx);
    }

    // 回退到软解
    if (!codec_ctx) {
        codec = avcodec_find_decoder(AV_CODEC_ID_H264);
        if (!codec) return NULL;
        
        codec_ctx = avcodec_alloc_context3(codec);
        if (!codec_ctx || avcodec_open2(codec_ctx, codec, NULL) < 0) {
            if (codec_ctx) avcodec_free_context(&codec_ctx);
            return NULL;
        }
    }

    d = malloc(sizeof(Decoder));
    d->codec_ctx = codec_ctx;
    d->frame = av_frame_alloc();
    d->sws_ctx = NULL;
    d->buffer = NULL;
    d->width = 1280;
    d->height = 720;
    d->sws_src_format = AV_PIX_FMT_YUV420P;
    d->hw_device_ctx = hw_device_ctx;

    return d;
}

static void free_decoder(Decoder *d) {
    if (d) {
        avcodec_close(d->codec_ctx);
        avcodec_free_context(&d->codec_ctx);
        av_frame_free(&d->frame);
        sws_freeContext(d->sws_ctx);
        av_free(d->buffer);
        free(d);
    }
}

static int decode_frame(Decoder *d, uint8_t *data, int size, uint8_t **out, int *w, int *h) {
	if (&data[0] == NULL)
		return;
    AVPacket pkt;
    av_init_packet(&pkt);
    pkt.data = data;
    pkt.size = size;

    int ret = avcodec_send_packet(d->codec_ctx, &pkt);
    av_packet_unref(&pkt);
	if (ret < 0) {
		printf("decode error3");
		return ret;
	}

      // 接收帧并处理硬件加速
    while (1) {
        ret = avcodec_receive_frame(d->codec_ctx, d->frame);
        if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF) {
			//printf("avcodec_receive_frame error %s\n", av_err2str(ret));
			//ret = 0;
			return ret;
		}
        if (ret < 0) {
			printf("decode error2");
			return ret;
		}

        // 硬件帧转换到系统内存
        AVFrame *sw_frame = d->frame;
        if (d->frame->hw_frames_ctx) {
            sw_frame = av_frame_alloc();
            if (av_hwframe_transfer_data(sw_frame, d->frame, 0) < 0) {
                av_frame_free(&sw_frame);
                continue;
            }
        }

        // 更新转换上下文（如果需要）
        if (!d->sws_ctx || d->width != sw_frame->width || 
            d->height != sw_frame->height || d->sws_src_format != sw_frame->format) {
            
            sws_freeContext(d->sws_ctx);
            d->sws_ctx = sws_getContext(
                sw_frame->width, sw_frame->height,
                sw_frame->format,
                sw_frame->width, sw_frame->height,
                AV_PIX_FMT_RGBA,
                SWS_BILINEAR, NULL, NULL, NULL
            );
            
            d->width = sw_frame->width;
            d->height = sw_frame->height;
            d->sws_src_format = sw_frame->format;
            
            av_free(d->buffer);
            d->buffer = av_mallocz(sw_frame->width * sw_frame->height * 4);
        }

        // 执行颜色空间转换
        uint8_t *dst[] = { d->buffer };
        int dst_linesize[] = { d->width * 4 };
        sws_scale(d->sws_ctx, sw_frame->data, sw_frame->linesize,
                  0, d->height, dst, dst_linesize);

        if (sw_frame != d->frame) {
            av_frame_unref(d->frame);
            av_frame_free(&sw_frame);
        }

        *out = d->buffer;
        *w = d->width;
        *h = d->height;
        return 0;
    }
    return ret;
}

// 新增音频解码器结构体
typedef struct {
    AVCodecContext *codec_ctx;
    struct SwrContext *swr_ctx;
    AVFrame *frame;
    int target_sample_rate;
    int target_channels;
    enum AVSampleFormat target_sample_fmt;
    
    // 新增重用缓冲区
    uint8_t **resampled_data;
    int resampled_linesize;
    int max_samples;
} AudioDecoder;

// 创建音频解码器
static AudioDecoder* create_audio_decoder(int sample_rate, int channels) {
    const AVCodec *codec = avcodec_find_decoder(AV_CODEC_ID_AAC);
    if (!codec) return NULL;

    AVCodecContext *codec_ctx = avcodec_alloc_context3(codec);
    if (!codec_ctx) return NULL;

    codec_ctx->sample_rate = sample_rate;
    codec_ctx->sample_fmt = AV_SAMPLE_FMT_FLTP;
    codec_ctx->sample_rate = 44100;
     // 设置解码器参数时增加容错
    codec_ctx->sample_rate = sample_rate;
    codec_ctx->bit_rate = 0; // 自动检测
    codec_ctx->strict_std_compliance = FF_COMPLIANCE_STRICT;
    codec_ctx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;


#if LIBAVCODEC_VERSION_MAJOR < 59
    codec_ctx->channels = channels;
    codec_ctx->channel_layout = av_get_default_channel_layout(channels);
#else
    //AVChannelLayout layout = AV_CHANNEL_LAYOUT_STEREO;
    AVChannelLayout layout;
    av_channel_layout_default(&layout, channels); // 自动推导标准布局
    av_channel_layout_copy(&codec_ctx->ch_layout, &layout);
#endif
    
    codec_ctx->codec_type = AVMEDIA_TYPE_AUDIO;

	
    //codec_ctx->flags &= ~AV_CODEC_FLAG_AC_PRED;
    
    // 设置正确的profile
    //codec_ctx->profile = FF_PROFILE_AAC_LOW;

    if (avcodec_open2(codec_ctx, codec, NULL) < 0) {
        avcodec_free_context(&codec_ctx);
        return NULL;
    }

    AudioDecoder *d = malloc(sizeof(AudioDecoder));
    if (!d) {
        avcodec_close(codec_ctx);
        avcodec_free_context(&codec_ctx);
        return NULL;
    }
    memset(d, 0, sizeof(AudioDecoder));

    d->codec_ctx = codec_ctx;
    d->frame = av_frame_alloc();
    d->target_sample_rate = sample_rate;
    d->target_channels = channels;
    d->target_sample_fmt = AV_SAMPLE_FMT_S16;

    return d;
}

// 在decode_audio_frame开头添加
static int total_decode_time = 0;
static int frame_count = 0;
// 解码音频帧并转换格式
static int decode_audio_frame(AudioDecoder *d, uint8_t *data, int size, uint8_t **out, int *out_size) {
	struct timeval start, end;
    gettimeofday(&start, NULL);
    AVPacket pkt;
    av_init_packet(&pkt);
    pkt.data = data;
    pkt.size = size;

    int ret = avcodec_send_packet(d->codec_ctx, &pkt);
    av_packet_unref(&pkt);
     if (ret < 0) {
        char errbuf[256];
        av_strerror(ret, errbuf, sizeof(errbuf));
        fprintf(stderr, "Error sending packet: %s\n", errbuf);
        return ret;
    }

    ret = avcodec_receive_frame(d->codec_ctx, d->frame);
    if (ret < 0) return ret;

    // 利用首次数据初始化重采样器
    if (!d->swr_ctx) {
        AVChannelLayout out_layout = AV_CHANNEL_LAYOUT_STEREO;
        swr_alloc_set_opts2(&d->swr_ctx,
                           &out_layout,                    // 输出声道布局
                           AV_SAMPLE_FMT_S16,              // 输出格式
                           44100,                         // 输出采样率
                           &d->frame->ch_layout,           // 输入声道布局
                           d->frame->format,               // 输入格式
                           d->frame->sample_rate,          // 输入采样率
                           0, NULL);
        swr_init(d->swr_ctx);
    }
    if (!d->swr_ctx || swr_init(d->swr_ctx) < 0) {
        if (d) {
			avcodec_close(d->codec_ctx);
			avcodec_free_context(&d->codec_ctx);
			av_frame_free(&d->frame);
			swr_free(&d->swr_ctx);
			free(d);
		}
        return NULL;
    }
    
    // 重采样
    // 重用缓冲区
    int need_realloc = 0;
    int max_samples = av_rescale_rnd(d->frame->nb_samples, 
                                    d->target_sample_rate, 
                                    d->codec_ctx->sample_rate, 
                                    AV_ROUND_UP);
                                    
    if (max_samples > d->max_samples) {
        need_realloc = 1;
        d->max_samples = max_samples * 2; // 预分配双倍空间
    }

    if (need_realloc || !d->resampled_data) {
        if (d->resampled_data) {
            av_freep(&d->resampled_data[0]);
            av_freep(&d->resampled_data);
        }
        
        av_samples_alloc_array_and_samples(&d->resampled_data, &d->resampled_linesize,
                                          d->target_channels, d->max_samples,
                                          d->target_sample_fmt, 0);
    }

    // 使用预分配缓冲区
    int converted_samples = swr_convert(d->swr_ctx, 
                                       d->resampled_data, d->max_samples,
                                       (const uint8_t **)d->frame->data, 
                                       d->frame->nb_samples);
    if (converted_samples < 0) {
        char errbuf[256];
        av_strerror(converted_samples, errbuf, sizeof(errbuf));
        fprintf(stderr, "SWR convert error: %s\n", errbuf);
        // ...释放资源...
        av_freep(&d->resampled_data[0]);
        av_freep(&d->resampled_data);
        return -1;
    }

    int data_size = converted_samples * d->target_channels * av_get_bytes_per_sample(d->target_sample_fmt);
    //uint8_t *pcm_copy = malloc(data_size);
    //if (!pcm_copy) {
    //    av_freep(&d->resampled_data[0]);
    //    av_freep(&d->resampled_data);
    //    return -1;
    //}
    //memcpy(pcm_copy, d->resampled_data[0], data_size);

    //av_freep(&d->resampled_data[0]);
    //av_freep(&d->resampled_data);

    //*out = pcm_copy;
    *out = d->resampled_data[0];
    *out_size = data_size;
    
    gettimeofday(&end, NULL);
    long decode_usec = (end.tv_sec - start.tv_sec)*1000000 + 
                      (end.tv_usec - start.tv_usec);
    total_decode_time += decode_usec;
    frame_count++;
    
    // 每50帧打印统计信息
    if (frame_count % 50 == 0) {
        fprintf(stderr, "[Audio] Avg decode time: %.2fms | Queue: %d/%d\n",
               (total_decode_time/1000.0)/frame_count,
               (audio_queue_head - audio_queue_tail + AUDIO_QUEUE_SIZE) % AUDIO_QUEUE_SIZE,
               AUDIO_QUEUE_SIZE);
        total_decode_time = 0;
        frame_count = 0;
    }
    
    return 0;
}

// 在全局变量区添加
#define AUDIO_BUFFER_SIZE (44100 * 2 * 2 / 5)
static uint8_t audio_buffer[AUDIO_BUFFER_SIZE];
static int audio_buffer_pos = 0;

// 全局音频解码器实例
static AudioDecoder *audio_decoder = NULL;

// 声明Go回调
extern void audioPCMCallback(void* data, int len);

static void process_single_frame(uint8_t* data, int len) {
    // 此处处理单个完整ADTS帧
    uint8_t *pcm_data = NULL;
    int pcm_size = 0;
    
    uint8_t adts_header[7];
    int profile = 1;
    int sample_rate_idx = 4;
    int channels = 2;
    int frame_length = len + 7;

    adts_header[0] = 0xFF;
    adts_header[1] = 0xF1;
    adts_header[2] = (profile << 6) | (sample_rate_idx << 2) | (channels >> 2);
    adts_header[3] = (channels << 6) | (frame_length >> 11);
    adts_header[4] = (frame_length >> 3) & 0xFF;
    adts_header[5] = ((frame_length & 0x07) << 5) | 0x1F;
    adts_header[6] = 0xFC;

    uint8_t *aac_data = malloc(len + 7);
    memcpy(aac_data, adts_header, 7);
    memcpy(aac_data + 7, data, len);

	if (!audio_decoder) {
        int sample_rate = 44100; // 根据sample_rate_idx转换
        audio_decoder = create_audio_decoder(sample_rate, 2);
        if (!audio_decoder) {
            fprintf(stderr, "无法创建音频解码器\n");
            return;
        }
    }

    int ret = decode_audio_frame(audio_decoder, aac_data, len+7, &pcm_data, &pcm_size);
    
    if (ret == 0 && pcm_data != NULL) {
        audioPCMCallback(pcm_data, pcm_size);
        //free(pcm_data);
    }
}

// 放入队列函数
static void enqueue_audio_packet(uint8_t *data, int len) {
    pthread_mutex_lock(&audio_queue_mutex);

    // 动态队列调整策略
    int queue_usage = (audio_queue_head - audio_queue_tail + AUDIO_QUEUE_SIZE) % AUDIO_QUEUE_SIZE;
    if (queue_usage > AUDIO_QUEUE_SIZE * 0.8) {
        // 当队列使用超过80%时，激进丢弃旧帧
        int drop_count = queue_usage / 2;
        while (drop_count-- > 0) {
            free(audio_packet_queue[audio_queue_tail].data);
            audio_queue_tail = (audio_queue_tail + 1) % AUDIO_QUEUE_SIZE;
        }
    }
    
    // 丢弃旧帧策略：当队列剩余空间小于10%时清空队列
    if ((audio_queue_head + 1) % AUDIO_QUEUE_SIZE == audio_queue_tail) {
        // 清空队列
        while (audio_queue_tail != audio_queue_head) {
            free(audio_packet_queue[audio_queue_tail].data);
            audio_queue_tail = (audio_queue_tail + 1) % AUDIO_QUEUE_SIZE;
        }
    }

    // 分配内存并拷贝数据
    AudioPacket pkt;
    pkt.data = malloc(len);
    pkt.size = len;
    memcpy(pkt.data, data, len);
    
    audio_packet_queue[audio_queue_head] = pkt;
    audio_queue_head = (audio_queue_head + 1) % AUDIO_QUEUE_SIZE;
    
    pthread_cond_signal(&audio_queue_cond);
    pthread_mutex_unlock(&audio_queue_mutex);
}

// 处理线程函数
static void* process_audio_frames(void* arg) {
    while (audio_processing) {
        pthread_mutex_lock(&audio_queue_mutex);
        
        while (audio_queue_tail == audio_queue_head && audio_processing) {
            pthread_cond_wait(&audio_queue_cond, &audio_queue_mutex);
        }
        
        if (!audio_processing) break;
        
        AudioPacket pkt = audio_packet_queue[audio_queue_tail];
        audio_queue_tail = (audio_queue_tail + 1) % AUDIO_QUEUE_SIZE;
        
        pthread_mutex_unlock(&audio_queue_mutex);
        
        // 调用原有处理逻辑
        process_single_frame(pkt.data, pkt.size);
        free(pkt.data);
    }
    return NULL;
}

// 初始化音频处理线程
static void init_audio_processing() {
    audio_processing = 1;
    pthread_t thread;
    pthread_create(&thread, NULL, process_audio_frames, NULL);
}

// 停止音频处理
static void stop_audio_processing() {
    pthread_mutex_lock(&audio_queue_mutex);
    audio_processing = 0;
    pthread_cond_signal(&audio_queue_cond);
    pthread_mutex_unlock(&audio_queue_mutex);
}

// 修改后的音频回调
static void audioStreamCallback(void* data, int len) {
	addaudio();
	if (len == 2) {
		//该2个字节为myt添加的标记 不用处理 
		return;
	}
    
    //process_single_frame(data, len);
    enqueue_audio_packet(data, len);
}


// 为简化调用，包装 startVideoStream 成为 start_stream 函数
static inline int start_stream(long handle, int w, int h, int bitrate) {
    return startVideoStream(handle, w, h, bitrate, videoStreamCallback, audioStreamCallback);
}

*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	//"io/ioutil"
	"sync"
	"time"
	"unsafe"
	"math"
	//"reflect"
	"flag"
	"os"
	"bytes"
	"mime/multipart"
	"io"
	"net/http"
	"strings"
	"path/filepath"
	"runtime"
	"encoding/json"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/theme"
	"github.com/disintegration/imaging"
	"github.com/hajimehoshi/oto"
)

var audiodelta = 0
var lastAudiodelta = 0
var videodelta = 0
var lastVideodelta = 0

//export addaudio
func addaudio() {
	if audiodelta < 1000000 {
		audiodelta++
	} else {
		audiodelta = 1
	}
}

// -------------------- SDK 封装模块 --------------------

// openDevice 封装 openDevice 接口
func openDevice(ip string, port int, timeout int) (C.long, error) {
	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))
	handle := C.openDevice(cip, C.int(port), C.long(timeout))
	if handle <= 0 {
		return 0, errors.New("openDevice 调用失败")
	}
	return handle, nil
}

// closeDevice 封装 closeDevice 接口
func closeDevice(handle C.long) error {
	ret := C.closeDevice(handle)
	if ret <= 0 {
		return errors.New("closeDevice 调用失败")
	}
	handle = 0;
	return nil
}

// startVideoStream 封装 startVideoStream 接口（通过我们在 C 中包装的 start_stream）
func startVideoStream(handle C.long, width, height, bitrate int) error {
	fmt.Println("start_stream start", handle)
	ret := C.start_stream(handle, C.int(width), C.int(height), C.int(bitrate))
	if ret != 1 {
		return errors.New("startVideoStream 调用失败")
	}
	fmt.Println("start_stream ret", ret)
	return nil
}

// stopVideoStream 封装 stopVideoStream 接口
func stopVideoStream(handle C.long) error {
	ret := C.stopVideoStream(handle)
	if ret != 1 {
		return errors.New("stopVideoStream 调用失败")
	}
	return nil
}

// takeScreenshot 封装 takeCaptrueCompress 接口，返回压缩后的截图数据（例如 PNG）
func takeScreenshot(handle C.long, compType int, quality int) ([]byte, error) {
	var length C.int
	ret := C.takeCaptrueCompress(handle, C.int(compType), C.int(quality), &length)
	if ret == nil {
		return nil, errors.New("takeCaptrueCompress 调用失败")
	}
	defer C.freeRpcPtr(unsafe.Pointer(ret))
	data := C.GoBytes(unsafe.Pointer(ret), length)
	return data, nil
}

// -------------------- 推流及 h264 解码（伪实现） --------------------

// generateDummyImage 生成一张纯色图像，供展示使用
func generateDummyImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// 生成随机颜色，这里固定为蓝色
	blue := color.RGBA{0, 0, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{blue}, image.Point{}, draw.Src)
	return img
}

type VideoFrame struct {
    Rotation int    // 0-3 对应 0°, 90°, 180°, 270°
    Data     []byte
}

type AudioFrame struct {
    Data     []byte
}

// -------------------- 推流及 h264 解码改进 --------------------
const (
	MaxFrameBuffer = 30 // 缓冲30帧防止卡顿
)

var (
	videoStreamChan  = make(chan VideoFrame, MaxFrameBuffer)
	projectionWindow fyne.Window
	projectionImg    *canvas.Image
	interactiveImg   *InteractiveImage
	currentRotation = -1
)

//export videoStreamCallback
func videoStreamCallback(rot C.int, data unsafe.Pointer, length C.int) {
	// 非阻塞方式发送，保留最新30帧
	select {
	case videoStreamChan <- VideoFrame{
		Rotation: int(rot),
		Data:     C.GoBytes(data, length),
	}:
		if videodelta < 1000000 {
			videodelta++
		} else {
			videodelta = 1
		}
		//fmt.Println("旋转改变1", currentRotation, int(rot))
		if currentRotation != int(rot) {
			currentRotation = int(rot)
			var currentWidth float32
			var currentHeight float32
			// 旋转改变，刷新窗口大小
			switch currentRotation {
				case 0, 2: // 竖屏模式
					currentWidth = 540
					currentHeight = 960
				default: // 横屏模式
					currentWidth = 960
					currentHeight = 540
			}
			//fmt.Println("旋转改变", currentRotation, currentWidth, currentHeight)
			projectionWindow.Resize(fyne.NewSize(currentWidth, currentHeight))
			//session.lock.Lock()
		
			// 获取原始大小用来计算图像随窗口等比缩放并维持在窗口中间
			orgWidth := float64(projectionImg.Image.Bounds().Dx())
			orgHeight := float64(projectionImg.Image.Bounds().Dy())
			
			// 计算缩放比例
			scale := math.Min(
				float64(currentWidth-8)/orgWidth,
				float64(currentHeight-8)/orgHeight,
			)

			// 计算实际显示尺寸和位置
			newWidth := float32(orgWidth * scale)
			newHeight := float32(orgHeight * scale)
			
			//fmt.Println("1大小改变", newWidth, newHeight)
			interactiveImg.Resize(fyne.NewSize(newWidth, newHeight))
			//main_content.Move(fyne.NewPos(size.Width/2-(newWidth/2), 0))
			interactiveImg.Refresh()
			//session.lock.Unlock()
			//projectionWindow
		}
	default:
		// 丢弃最旧帧
		<-videoStreamChan
		videoStreamChan <- VideoFrame{
			Rotation: int(rot),
			Data:     C.GoBytes(data, length),
		}
	}
}

// 图像旋转函数
func rotateImage(src image.Image, rotation int) image.Image {
	//fmt.Println("角度2", rotation)
    switch rotation {
    case 2: // 90°
        return imaging.Rotate90(src)
    case 3: // 180°
        return imaging.Rotate180(src)
    case 0: // 270°
        return imaging.Rotate270(src)
    default: // 0°
        return src
    }
}

func UNUSED(x ...interface{}) {}

type H264Decoder struct {
	decoder   *C.Decoder
	lastWidth  int
	lastHeight int
	imgBuffer  *image.RGBA
}

func NewH264Decoder() (*H264Decoder, error) {
	d := C.create_decoder()
	if d == nil {
		return nil, fmt.Errorf("无法创建解码器")
	}
	return &H264Decoder{decoder: d}, nil
}

func (d *H264Decoder) Close() {
	C.free_decoder(d.decoder)
}

func (d *H264Decoder) Decode(data []byte) (image.Image, error) {
	var out *C.uint8_t
	var w, h C.int

	ret := C.decode_frame(d.decoder, (*C.uint8_t)(&data[0]), C.int(len(data)), &out, &w, &h)
	// 处理正常等待情况
	if ret == 1 {
		return nil, nil // 正常等待更多数据
	}
	if ret != 0 {
		return nil, fmt.Errorf("解码失败: %d", ret)
	}

	currentWidth := int(w)
	currentHeight := int(h)

	// 检查尺寸变化
	if d.lastWidth != currentWidth || d.lastHeight != currentHeight {
		d.imgBuffer = image.NewRGBA(image.Rect(0, 0, currentWidth, currentHeight))
		d.lastWidth = currentWidth
		d.lastHeight = currentHeight
	}

	// 完整复制图像数据
	rgb := C.GoBytes(unsafe.Pointer(out), C.int(currentWidth*currentHeight*4))
	copy(d.imgBuffer.Pix, rgb)

	return d.imgBuffer, nil
}

func addADTSHeader(frame []byte, sampleRate, channels int) []byte {
    profile := 1 // AAC LC
    sampleRateIndex := 4 // 44100 Hz
    channelConfig := channels
    frameLength := len(frame) + 7

    adtsHeader := make([]byte, 7)
    // Syncword 12 bits: 0xFFF
    adtsHeader[0] = 0xFF
    adtsHeader[1] = 0xF0 
    // Profile 2 bits, sample rate index 4 bits
    adtsHeader[2] = byte((profile-1)<<6) | byte(sampleRateIndex<<2)
    // Channel config 3 bits, frame length 13 bits
    adtsHeader[3] = byte(channelConfig<<6) | byte((frameLength>>11)&0x03)
    adtsHeader[4] = byte((frameLength >> 3) & 0xFF)
    adtsHeader[5] = byte((frameLength&0x07)<<5) | 0x1F
    adtsHeader[6] = 0xFC

    return append(adtsHeader, frame...)
}

var (
    audioPlayer *oto.Player
    audioContext *oto.Context
)

// 使用内存池避免GC停顿
var pcmPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096) 
    },
}

//export audioPCMCallback
func audioPCMCallback(data unsafe.Pointer, length C.int) {
    buf := pcmPool.Get().([]byte)
    defer pcmPool.Put(buf[:0])
    
    // 重用缓冲区（零拷贝）
    buf = (*[1 << 28]byte)(data)[:length:length]
    
    if audioPlayer != nil {
        audioPlayer.Write(buf)
    }
}

func initAudio() error {
    // 参数与C代码中的目标格式一致：44100Hz, 2声道, 16位
    var err error
    // 将缓冲区从4KB增加到40KB（约200ms缓冲）
    const bufferSize = 44100 * 2 * 2 / 5 // 44100Hz * 16bit * 2ch /5 = 35280 bytes
    audioContext, err := oto.NewContext(44100, 2, 2, bufferSize)
    if err != nil {
        return err
    }
    audioPlayer = audioContext.NewPlayer()
    C.init_audio_processing()
    return nil
}

// 修改后的解码显示函数
func decodeAndDisplayVideo(img *canvas.Image, stopChan <-chan struct{}, wg *sync.WaitGroup) {
    defer wg.Done()

    decoder, err := NewH264Decoder()
    if err != nil {
        fmt.Println("创建解码器失败:", err)
        return
    }
    defer decoder.Close()

	for {
		select {
			case frame := <-videoStreamChan:
				decodedImg, err := decoder.Decode(frame.Data)
				if err != nil {
					fmt.Println("解码错误:", err)
					continue
				}
				if decodedImg == nil {
					continue // 正常等待数据
				}
				//fmt.Println("当前旋转:", frame.Rotation)

				// 应用旋转并显示
				rotatedImg := rotateImage(decodedImg, frame.Rotation)
				img.Image = rotatedImg
				img.Refresh()
			//case data := <-audioStreamChan:
			//	fmt.Println("音频", data)
			//	adtsData := addADTSHeader(data, 44100, 2)
			//	ioutil.WriteFile("audio.aac", adtsData, 0644)
			default:

		case <-stopChan:
			return
		}
	}
}

// -------------------- GUI 改进 --------------------
var (
	currentTicker    *time.Ticker
	currentStopChan  chan struct{}
	intervalSelect   *widget.Select
)

// -------------------- GUI 与业务逻辑 --------------------

var (
	discoveredDevices []string
	selectedDevice    string
	sdkHandle         C.long
	stopStreamChan    chan struct{}
	wg                sync.WaitGroup
)

func (s *TouchSession) mapCoordinates(pos fyne.Position) (x, y int) {
    s.lock.RLock()
    defer s.lock.RUnlock()

    if s.windowSize.Width == 0 || s.windowSize.Height == 0 {
        return 0, 0
    }
    
    s.rotation = currentRotation

    // 计算缩放比例（考虑设备旋转）
    var scaleX, scaleY float64
    switch s.rotation {
    case 0, 2: // 竖屏模式
        scaleX = float64(s.sourceSize.Height) / float64(s.windowSize.Width)
        scaleY = float64(s.sourceSize.Width) / float64(s.windowSize.Height)
    default: // 横屏模式
        scaleX = float64(s.sourceSize.Width) / float64(s.windowSize.Width)
        scaleY = float64(s.sourceSize.Height) / float64(s.windowSize.Height)
    }

    // 坐标映射
    switch s.rotation {
    case 0: // 设备270°旋转
        return int(float64(pos.X) * scaleX), int(float64(pos.Y)*scaleY)            
    case 3: // 180°
        return int(float64(s.sourceSize.Width) - float64(pos.X)*scaleX),
            int(float64(s.sourceSize.Height) - float64(pos.Y)*scaleY)
    case 2: // 90°
        return int(float64(pos.Y) * scaleY),
            int(float64(s.sourceSize.Width) - float64(pos.X)*scaleX)
    default: // 0°
        return int(float64(pos.X) * scaleX),
            int(float64(pos.Y) * scaleY)
    }
}

// 检查并维持连接
func (s *TouchSession) checkHandle(ipaddr string, port int, bakport int) error {
    if s.handle <= 0 {
        handle, err := openDevice(ipaddr, port, 3)
        if err != nil {
			handle, err := openDevice(ipaddr, bakport, 3)
			if err != nil {
				return err
			} else {
				s.handle = C.long(handle)
			}
        } else {
			s.handle = C.long(handle)
        }
    }
    return nil
}

type InteractiveImage struct {
    *canvas.Image
    onTapped           func(pos fyne.Position)
    onTappedSecondary  func(pos fyne.Position)
    onScrolled         func(ev *fyne.ScrollEvent)
    onDragged		   func(d *fyne.DragEvent)
    onDragEnd		   func()
}

func NewInteractiveImage(img *canvas.Image) *InteractiveImage {
    return &InteractiveImage{Image: img}
}

// 实现Tappable接口
func (i *InteractiveImage) Tapped(ev *fyne.PointEvent) {
    if i.onTapped != nil {
        i.onTapped(ev.Position)
    }
}

// 实现TappedSecondary接口
func (i *InteractiveImage) TappedSecondary(ev *fyne.PointEvent) {
    if i.onTappedSecondary != nil {
        i.onTappedSecondary(ev.Position)
    }
}

// 实现Scrollable接口
func (i *InteractiveImage) Scrolled(ev *fyne.ScrollEvent) {
    if i.onScrolled != nil {
        i.onScrolled(ev)
    }
}

// 实现Dragged接口
func (i *InteractiveImage) Dragged(ev *fyne.DragEvent) {
    if i.onDragged != nil {
        i.onDragged(ev)
    }
}

// 实现DragEnd接口
func (i *InteractiveImage) DragEnd() {
    if i.onDragEnd != nil {
        i.onDragEnd()
    }
}

func (b *InteractiveImage) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.Image)
}

type TouchSession struct {
    handle       C.long
    rotation     int
    windowSize   fyne.Size
    sourceSize   fyne.Size
    touchPoints  map[int]fyne.Position
    lock         sync.RWMutex
}

func NewTouchSession(sourceW, sourceH int) *TouchSession {
    return &TouchSession{
        sourceSize:  fyne.NewSize(float32(sourceW), float32(sourceH)),
        touchPoints: make(map[int]fyne.Position),
    }
}

type SizeWatcher struct {
    widget.BaseWidget
    OnSizeChanged func(fyne.Size)
}

func NewSizeWatcher(callback func(fyne.Size)) *SizeWatcher {
    w := &SizeWatcher{OnSizeChanged: callback}
    w.ExtendBaseWidget(w)
    return w
}

func (w *SizeWatcher) CreateRenderer() fyne.WidgetRenderer {
    return &sizeWatcherRenderer{
        widget: w,
        rect:   canvas.NewRectangle(color.Transparent),
    }
}

type sizeWatcherRenderer struct {
    widget *SizeWatcher
    rect   *canvas.Rectangle
}

func (r *sizeWatcherRenderer) Layout(size fyne.Size) {
    r.rect.Resize(size)
    if r.widget.OnSizeChanged != nil {
        r.widget.OnSizeChanged(size)
    }
}

func (r *sizeWatcherRenderer) MinSize() fyne.Size     { return fyne.NewSize(0, 0) }
func (r *sizeWatcherRenderer) Refresh()               {}
func (r *sizeWatcherRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *sizeWatcherRenderer) Destroy()               {}

func setupInputHandlers(img *InteractiveImage, session *TouchSession) {
    // 点击事件
    img.onTapped = func(pos fyne.Position) {
		//fmt.Println("点击")
        x, y := session.mapCoordinates(pos)
        
		if audiodelta != 0 && lastAudiodelta == audiodelta && lastVideodelta == videodelta {
			fmt.Println("重连", audiodelta, lastAudiodelta)
			C.stopVideoStream(C.long(session.handle))
			startVideoStream(C.long(session.handle), 1280, 720, 2000);
		}
		lastAudiodelta = audiodelta
		lastVideodelta = videodelta
        C.touchClick(session.handle, 0, C.int(x), C.int(y))
        //C.touchUp(session.handle, 0, C.int(x), C.int(y))
    }
    
    // 右键点击
    img.onTappedSecondary = func(pos fyne.Position) {
		//fmt.Println("右键点击")
		if audiodelta != 0 && lastAudiodelta == audiodelta && lastVideodelta == videodelta {
			fmt.Println("重连", audiodelta, lastAudiodelta)
			C.stopVideoStream(C.long(session.handle))
			startVideoStream(C.long(session.handle), 1280, 720, 2000);
		}
		lastAudiodelta = audiodelta
		lastVideodelta = videodelta
        // 右键返回
        C.keyPress(session.handle, 4)
    }

    // 滚动事件
    img.onScrolled = func(ev *fyne.ScrollEvent) {
		if audiodelta != 0 && lastAudiodelta == audiodelta && lastVideodelta == videodelta {
			fmt.Println("重连", audiodelta, lastAudiodelta)
			C.stopVideoStream(C.long(session.handle))
			startVideoStream(C.long(session.handle), 1280, 720, 2000);
		}
		lastAudiodelta = audiodelta
		lastVideodelta = videodelta
		//fmt.Println("滚动", ev)
        startX, startY := session.mapCoordinates(ev.Position)
        delta := int(ev.Scrolled.DY * 20)
        C.swipe(session.handle, 0, 
            C.int(startX), C.int(startY),
            C.int(startX), C.int(startY-delta),
            C.long(100), C.bool(false))
    }
    
    // 滑动事件
    img.onDragged = func(d *fyne.DragEvent) {
		x, y := session.mapCoordinates(d.PointEvent.Position)
        if _, ok := session.touchPoints[0]; !ok {
            if audiodelta != 0 && lastAudiodelta == audiodelta && lastVideodelta == videodelta {
				C.stopVideoStream(C.long(session.handle))
				fmt.Println("重连", audiodelta, lastAudiodelta)
				startVideoStream(C.long(session.handle), 1280, 720, 2000);
			}
			lastAudiodelta = audiodelta
			lastVideodelta = videodelta
            C.touchDown(session.handle, 0, C.int(x), C.int(y))
        } else {
            C.touchMove(session.handle, 0, C.int(x), C.int(y))
        }
        session.touchPoints[0] = d.PointEvent.Position
    }
    
    // 滑动结束
    img.onDragEnd = func() {
        if audiodelta != 0 && lastAudiodelta == audiodelta && lastVideodelta == videodelta {
			C.stopVideoStream(C.long(session.handle))
			fmt.Println("重连", audiodelta, lastAudiodelta)
			startVideoStream(C.long(session.handle), 1280, 720, 2000);
		}
		lastAudiodelta = audiodelta
		lastVideodelta = videodelta
		//fmt.Println("拖动结束")
		x, y := session.mapCoordinates(session.touchPoints[0])
		C.touchUp(session.handle, 0, C.int(x), C.int(y))
		session.touchPoints = make(map[int]fyne.Position)
    }
}

type ExecResult struct {
    ExitCode int
    Stdout   string
    Stderr   string
}

func sanitizeString(s string) string {
    return strings.ToValidUTF8(strings.Map(func(r rune) rune {
        if unicode.IsControl(r) && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, s), "�")
}

func dockerExec(ip string, port int, containerID string, cmd []string) (*ExecResult, error) {
    // 1. 创建exec实例
    createURL := fmt.Sprintf("http://%s:%d/containers/%s/exec", ip, port, containerID)
    reqBody := map[string]interface{}{
        "AttachStdout": true,
        "AttachStderr": true,
        "Cmd":          cmd,
    }
    
    resp, err := postJSON(createURL, reqBody)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var execCreate struct{ Id string }
    if err := json.NewDecoder(resp.Body).Decode(&execCreate); err != nil {
        return nil, err
    }

    // 2. 启动exec并捕获输出
    startURL := fmt.Sprintf("http://%s:%d/exec/%s/start", ip, port, execCreate.Id)
    resp, err = postJSON(startURL, map[string]interface{}{
        "Detach": false,
        "Tty":    false,
    })
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // 3. 读取流式输出（Docker API帧格式）
    var (
        stdout, stderr bytes.Buffer
        rawResponse    bytes.Buffer
    )
    tee := io.TeeReader(resp.Body, &rawResponse)
    dec := json.NewDecoder(tee)
    for dec.More() {
        var frame struct {
            Type string `json:"Type"`
            Body string `json:"Body"`
        }
        if err := dec.Decode(&frame); err != nil {
            //fmt.Println("JSON解析失败，原始数据: %q", rawResponse.String())
            //return nil, fmt.Errorf("响应解析失败: %v", err)
            break
        }

        cleaned := sanitizeString(frame.Body)
        switch frame.Type {
        case "stdout":
            stdout.WriteString(cleaned)
        case "stderr":
            stderr.WriteString(cleaned)
        }
    }

    // 检查未解析数据
    //if rest, _ := io.ReadAll(dec.Buffered()); len(rest) > 0 {
    //    fmt.Println("未解析的响应数据: %q", rest)
    //}

    // 4. 获取退出码
    inspectURL := fmt.Sprintf("http://%s:%d/exec/%s/json", ip, port, execCreate.Id)
    resp, err = http.Get(inspectURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var execInspect struct {
        ExitCode int `json:"ExitCode"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&execInspect); err != nil {
        return nil, err
    }

    return &ExecResult{
        ExitCode: execInspect.ExitCode,
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
    }, nil
}

// 通用JSON POST请求
func postJSON(url string, data interface{}) (*http.Response, error) {
    body := new(bytes.Buffer)
    if err := json.NewEncoder(body).Encode(data); err != nil {
        return nil, err
    }
    
    req, err := http.NewRequest("POST", url, body)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    
    return http.DefaultClient.Do(req)
}

func uploadFile(ip string, port int, filePath string) error {
    // 打开文件
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()

    // 创建multipart请求
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, err := writer.CreateFormFile("file", filepath.Base(filePath))
    if err != nil {
        return err
    }

    // 拷贝文件内容
    if _, err = io.Copy(part, file); err != nil {
        return err
    }
    writer.Close()

    // 创建请求
    url := fmt.Sprintf("http://%s:%d/upload", ip, port)
    req, err := http.NewRequest("POST", url, body)
    if err != nil {
        return err
    }

    // 设置请求头
    req.Header.Set("Content-Type", writer.FormDataContentType())

    // 发送请求
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // 检查响应
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("服务器错误: %s", string(body))
    }
    return nil
}

func handleAPKInstall(ip string, uploadPort int, containerID string, 
    dockerPort int, localPath string) error {
    
    // 1. 上传文件
    if err := uploadFile(ip, uploadPort, localPath); err != nil {
        return fmt.Errorf("上传失败: %v", err)
    }

    // 2. 准备容器内路径
    fileName := filepath.Base(localPath)
    uploadPath := "/sdcard/upload/" + fileName
    installPath := "/data/local/tmp/" + fileName
    
    // 修改命令构造方式，确保路径正确转义
	//installPath := fmt.Sprintf("/data/local/tmp/%s", shellEscape(filepath.Base(localPath)))


    // 3. 执行容器内命令
	commands := []struct{
		Cmd     string
		Message string
	}{
		{fmt.Sprintf("mv '%s' '%s'", uploadPath, installPath), "移动文件中..."},
		{fmt.Sprintf("pm install -r '%s'", installPath), "安装APK中..."},
		{fmt.Sprintf("rm -f '%s'", installPath), "清理临时文件..."},
	}

    //var fullOutput strings.Builder
    for _, step := range commands {
        //updateProgress(step.Message)
        
        result, err := dockerExec(ip, dockerPort, containerID, []string{"sh", "-c", step.Cmd})
        if err != nil {
            return fmt.Errorf("执行失败: %v", err)
        }

        // 记录完整输出
        //fullOutput.WriteString(fmt.Sprintf(
        //    "[CMD] %s\nEXIT CODE: %d\nSTDOUT: %s\nSTDERR: %s\n\n",
        //    step.Cmd, result.ExitCode, result.Stdout, result.Stderr,
        //))

        if result.ExitCode != 0 {
            //showErrorDialog("命令执行失败",
            //    fmt.Sprintf("命令: %s\n退出码: %d\n输出:\n%s\n错误:\n%s",
            //        step.Cmd, result.ExitCode, result.Stdout, result.Stderr))
            return fmt.Errorf("命令 %s 失败 (退出码%d)", step.Cmd, result.ExitCode)
        }
    }
    
    return nil
}

func showInfoDialog(title, message string) {
    dialog.ShowCustom(
        title,
        "关闭",
        container.NewVBox(
            widget.NewLabel(message),
            widget.NewIcon(theme.ConfirmIcon()),
        ),
        projectionWindow,
    )
}

func showErrorDialog(title, message string) {
    dialog.ShowCustom(
        title,
        "关闭",
        container.NewVBox(
            widget.NewLabel(message),
            widget.NewIcon(theme.ErrorIcon()),
        ),
        projectionWindow,
    )
}

func showError(msg string) {
    //Edialog = widget.NewModalPopUp(
    //    container.NewHBox(
    //        widget.NewLabel("错误信息:"),
    //        widget.NewLabel(msg),
    //        widget.NewButton("关闭", func() { Edialog.Hide() }),
    //    ),
    //    mainWindow.Canvas(),
    //)
    Edialog := dialog.NewError(fmt.Errorf(msg), projectionWindow)
    Edialog.Show()
}

func main() {
    var name string
    
	ipaddr := flag.String("ip", "127.0.0.1", "设备IP地址")
    port := flag.Int("port", 9083, "控制端口")
    uploadPort := flag.Int("uploadPort", 9082, "上传API端口") // 新增上传端口
    containerID := flag.String("containerID", "", "Docker容器ID")
    width := flag.Int("width", 720, "宽度") // 新增上传端口
    height := flag.Int("height", 1280, "高度") // 新增上传端口
    bakport := flag.Int("bakport", 0, "高度") // 新增上传端口
    flag.StringVar(&name, "name", "名称", "name")

	flag.Parse()

	if ipaddr == nil || port == nil || uploadPort == nil || len(*ipaddr) == 0 || *port <= 0 || *uploadPort <= 0 || *containerID == "" {
		fmt.Println("缺少必要参数：ip, port, uploadPort, containerID")
		return
	}

	myApp := app.New()

	// 创建新窗口
	projectionWindow = myApp.NewWindow("投屏" + *ipaddr + " - " + name)
	projectionImg = canvas.NewImageFromImage(generateDummyImage(270,480))
	projectionImg.FillMode = canvas.ImageFillStretch
	interactiveImg = NewInteractiveImage(projectionImg)
	interactiveImg.FillMode = canvas.ImageFillStretch

	// 初始化触摸会话
    session := NewTouchSession(*height, *width) // 注意实际分辨率与旋转方向

    // 获取端口并连接
    //session.handle = sdkHandle
    err := session.checkHandle(*ipaddr, *port, *bakport)
    if err != nil {
		showError("连接失败")
		return
    }
	
	// 绑定事件处理
    setupInputHandlers(interactiveImg, session)
	
	stopChan := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	
	// 初始化音频播放
    if err := initAudio(); err != nil {
        fmt.Println("无法初始化音频播放:", err)
        return
    }
	
	// 启动视频流
	startVideoStream(C.long(session.handle), 1280, 720, 2000)

	go decodeAndDisplayVideo(projectionImg, stopChan, &wg)

	main_content := container.NewMax(interactiveImg)
	// 创建右侧功能栏
    funcPanel := container.NewHBox(
        widget.NewButton("🔊+", func() {
            C.keyPress(session.handle, 24)
        }),
        widget.NewButton("🔊-", func() {
            C.keyPress(session.handle, 25)
        }),
        widget.NewButton("🏠", func() {
            C.keyPress(session.handle, 3)
        }),
        widget.NewButton("↩", func() {
            C.keyPress(session.handle, 4)
        }),
    )

    // 修改界面布局
    mainContent := container.NewBorder(
        nil, funcPanel, nil, nil, // 底部放置功能栏
        main_content,
    )
	
	//newWidth := float32(0);
	// 创建尺寸监听组件
    sizeWatcher := NewSizeWatcher(func(size fyne.Size) {
        session.lock.Lock()
		
		// 获取原始大小用来计算图像随窗口等比缩放并维持在窗口中间
		orgWidth := float64(projectionImg.Image.Bounds().Dx())
		orgHeight := float64(projectionImg.Image.Bounds().Dy())
		
		// 计算缩放比例
		scale := math.Min(
			float64(size.Width)/orgWidth,
			float64(size.Height)/orgHeight,
		)

		// 计算实际显示尺寸和位置
		newWidth := float32(orgWidth * scale)
		newHeight := float32(orgHeight * scale)
		
		//fmt.Println("1大小改变", newWidth, newHeight, size)
		interactiveImg.Resize(fyne.NewSize(newWidth, newHeight))
		main_content.Move(fyne.NewPos(size.Width/2-(newWidth/2), 0))
		interactiveImg.Refresh()
        session.windowSize = fyne.NewSize(newWidth, newHeight)
        session.lock.Unlock()
    })

	// 组合容器
    content := container.NewStack(
        mainContent,
        sizeWatcher,
    )

	// 窗口关闭时清理
	projectionWindow.SetOnClosed(func() {
		close(stopChan)
		wg.Wait()
		closeDevice(C.long(session.handle))
		session.handle = 0
		<-videoStreamChan
		C.stop_audio_processing()
	})
	
	// 按键输入
	projectionWindow.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		//fmt.Println("按键输入", ev.Name)
		// 处理功能键
		switch ev.Name {
		case fyne.KeyReturn:
			C.keyPress(session.handle, 66)
		case fyne.KeyBackspace:
			C.keyPress(session.handle, 67)
		case fyne.KeyDelete:
			C.keyPress(session.handle, 112)
		case fyne.KeyLeft:
			C.keyPress(session.handle, 21) // KEYCODE_DPAD_LEFT
		case fyne.KeyRight:
			C.keyPress(session.handle, 22) // KEYCODE_DPAD_RIGHT
		// 可根据需要添加更多按键映射
		}
	})

	projectionWindow.Canvas().SetOnTypedRune(func(r rune) {
		// 处理字符输入
		cstr := C.CString(string(r))
		//fmt.Println("文本输入", cstr)
		defer C.free(unsafe.Pointer(cstr))
		C.sendText(session.handle, cstr)
	})
	
	// 文件上传
	projectionWindow.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		go func() { // 异步处理防止界面卡顿
			if len(uris) == 0 {
				return
			}

			// 显示上传提示
			uploadIndicator := canvas.NewText("上传中...", color.White)
			uploadIndicator.Move(fyne.NewPos(10, 10))
			//projectionWindow.Canvas().Add(uploadIndicator)
			//defer projectionWindow.Canvas().Remove(uploadIndicator)

			for _, uri := range uris {
				filePath := uri.Path()
				// 转换Windows路径格式（如果需要）
				cleanPath := strings.Replace(filePath, "file://", "", 1)
				cleanPath = strings.Replace(cleanPath, "%20", " ", -1)

				// Windows系统需要特殊处理
				if runtime.GOOS == "windows" {
					filePath = strings.TrimPrefix(cleanPath, "/")
					filePath = strings.Replace(cleanPath, "/", "\\", -1)
				}

				if strings.ToLower(filepath.Ext(filePath)) == ".apk" {
					// APK安装流程
					err := handleAPKInstall(
						*ipaddr, 
						*uploadPort,
						*containerID,
						2375,
						filePath,
					)
					if err != nil {
						showErrorDialog("安装失败", err.Error())
					} else {
						showInfoDialog("安装成功", filepath.Base(cleanPath))
					}
					// [错误处理...]
				} else {
					// 调用上传函数
					err := uploadFile(*ipaddr, *uploadPort, filePath)
					if err != nil {
						showErrorDialog("上传失败", err.Error())
					} else {
						showInfoDialog("上传成功：/sdcard/upload/", filepath.Base(cleanPath))
					}
				}
			}
		}()
	})
	
	projectionWindow.SetContent(content)
	//projectionWindow.SetContent(container.NewScroll(projectionImg))
	projectionWindow.Resize(fyne.NewSize(540, 960))
	projectionWindow.ShowAndRun()
}
