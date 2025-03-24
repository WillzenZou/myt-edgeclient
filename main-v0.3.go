// main.go
package main

/*
#cgo CFLAGS: -IE:\MYT\mytrpa\include
#cgo LDFLAGS: -L${SRCDIR} -lmytrpc
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include "libmytrpc.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
	"image/png"
	"bytes"
	"os"
	"os/exec"
	"sort"
	"encoding/base64"
    "net/url"
    "context"
    "runtime"
    "math/big"
    //"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"fyne.io/fyne/v2/dialog"
	"gitee.com/zoums/dget"
)


// -------------------- 类型定义和全局变量 --------------------
type DeviceInfo struct {
	IP       string
	Type     string
	ID       string
	Name     string
	Response string
}

type NetworkConfig struct {
    Subnet    string `json:"subnet"`
    Gateway   string `json:"gateway"`
    ParentIf  string `json:"parent_if"` // 父接口如eth0
    NetworkID string `json:"network_id"` // 记录网络ID
}

type PortInfo struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

// ContainerInfo 定义了从 Docker API 获取的容器信息
type ContainerInfo struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Ports  []PortInfo
	State  string `json:"State"`
	Status string `json:"Status"`
	Image  string `json:"Image"`
	Labels map[string]string `json:"Labels"`
	Command        string          `json:"Command"`
	HostConfig struct {
        Binds         []string          `json:"Binds"`
        PortBindings  map[string][]PortBinding `json:"PortBindings"`
        Devices       []DeviceMapping   `json:"Devices"`
        RestartPolicy struct {
            Name string `json:"Name"`
        } `json:"RestartPolicy"`
    } `json:"HostConfig"`
    Idx     int
    Width   int    // 容器分辨率宽度
    Height  int    // 容器分辨率高度
    RpaPort int    // 备用端口
    ImageName string `json:"image_name"`
}

// 新增创建容器请求结构体
type CreateContainerRequest struct {
    Image      string            `json:"Image"`
    Hostname   string            `json:"Hostname"`
    Labels     map[string]string `json:"Labels"`
    Env        []string          `json:"Env"`
    Cmd        []string          `json:"Cmd"`
    HostConfig struct {
        Binds         []string          `json:"Binds"`
        PortBindings  map[string][]PortBinding `json:"PortBindings"`
        Devices       []DeviceMapping   `json:"Devices"`
        DeviceCgroupRules []string      `json:"DeviceCgroupRules"`
        RestartPolicy struct {
            Name string `json:"Name"`
        } `json:"RestartPolicy"`
		CapAdd        []string			`json:"CapAdd"`
        SecurityOpt   []string			`json:"SecurityOpt"`
        Sysctls       map[string]string `json:"Sysctls"`
        NetworkMode   string            `json:"NetworkMode"`
    } `json:"HostConfig"`
    ExposedPorts  map[string]PortBinding `json:"ExposedPorts"`
    NetworkingConfig struct {
        EndpointsConfig map[string]interface{} `json:"EndpointsConfig"`
    } `json:"NetworkingConfig"`
}

type PortBinding struct {
    HostPort string `json:"HostPort"`
}

type DeviceMapping struct {
    PathOnHost        string `json:"PathOnHost"`
    PathInContainer   string `json:"PathInContainer"`
    CgroupPermissions string `json:"CgroupPermissions"`
}

// 新增全局变量
var (
    //existingIdxs = make(map[int]bool)
    deviceIdxs = make(map[string]map[int]bool)
    nextIdx    = 1
    Cdialog    *widget.PopUp
    deviceNetConfigs = make(map[string]NetworkConfig) // key为设备IP
    networkMutex     sync.RWMutex
)

type ContainerCard struct {
    widget.BaseWidget
    info       *ContainerInfo
    deviceIP   string
    img        *canvas.Image
    status     *widget.Label
    checkbox   *widget.Check
    updateChan chan bool
    running    bool
    mu         sync.Mutex
    lastImage  image.Image // 添加最后成功加载的图像缓存
    handle     C.long
    stopChan    chan struct{} // 改用stopChan
    active      bool
}

var (
	mainApp            fyne.App
	mainWindow        fyne.Window
	deviceList         *widget.List
	currentView      = "list"
	viewSwitch       *widget.Select
	contentStack     *container.TabItem
	listView         *widget.List
	gridView         *fyne.Container
	discoveredDevices []DeviceInfo
	currentContainers []ContainerInfo
	selectedDevice    string
	containerCards    = make(map[string]*ContainerCard)
	refreshTicker     *time.Ticker
    screenshotQueue    chan *ContainerCard // 截图任务队列
    currentInterval    = 1                 // 默认1秒
    intervalSelect     *widget.Select
    screenshotStopChan = make(chan struct{})
	allDevices []DeviceInfo
	filteredDevices []DeviceInfo
)

// 添加多选支持结构体
type SelectableDevice struct {
    DeviceInfo
    Selected bool
}

var (
    selectedDevices   = make(map[string]DeviceInfo)  // 选中的设备
    createSelectedDevices   = make(map[string]DeviceInfo)  // 创建时选中的设备
    selectedContainers = make(map[string]ContainerInfo) // 选中的容器
    allSelected      bool
    selectionLock    sync.Mutex
    selectAllCheck   *widget.Check
    progressBar   *widget.ProgressBar
    progressLabel *widget.Label
    pullDialog    *widget.PopUp
)

// 在CreateContainerRequest结构体后添加镜像下载进度结构体
type ImagePullStatus struct {
    Status         string `json:"status"`
    ProgressDetail struct {
        Current int64 `json:"current"`
        Total   int64 `json:"total"`
    } `json:"progressDetail"`
    Progress string `json:"progress"`
    Id       string `json:"id"`
}

// 任务队列
var (
    taskQueue      = make(chan CreateTask, 100) // 任务队列
    taskList       []*TaskItem                 // 当前任务列表
    taskWindow     *widget.PopUp               // 悬浮窗口
    taskListBox    *fyne.Container             // 任务列表容器
    queueLock      sync.Mutex                  // 队列锁
    taskId         = 0
)

// 添加任务结构体定义
type CreateTask struct {
    Device     DeviceInfo
    Name       string
    Resolution string
    Network string
    Ipaddr string
    ImageName  string
    TaskId     int
    UseLocalImage bool
}

type TaskItem struct {
    DeviceIP  string
    Status    string // waiting | pulling | creating | done | error
    Progress  float64
    Cancel    chan struct{}
    isStop    bool
    InfoLabel *widget.Label
    ProgressBar *widget.ProgressBar
    TaskId    int
}

type MytAndroidImage struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    URL    string   `json:"url"`
    TType  string   `json:"ttype"`
    TType2 []string `json:"ttype2"`
}

var (
    fullImageCache     []MytAndroidImage
    filteredImageCache []MytAndroidImage
    imageCacheLock sync.RWMutex
    lastImageFetch time.Time
    currentDeviceType = "C1" // 默认值
    typeOptions       = []string{"m48", "q1_10", "q1", "p1", "C1", "c1_10", "a1"}
)

// -------------------- UDP 发现模块 --------------------
// discoverDevices 使用 UDP 广播 "lgcloud" 到 7678 端口，并收集响应中带 ":" 的设备 IP
func discoverDevices() ([]DeviceInfo, error) {
    var devices []DeviceInfo

    fmt.Printf("start discoverDevices\n")

    // 获取所有网络接口
    interfaces, err := net.Interfaces()
    if err != nil {
        return nil, fmt.Errorf("获取网络接口失败: %v", err)
    }

    var broadcastAddrs []net.IP
    for _, iface := range interfaces {
        // 排除未启用的接口和回环接口
        if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
            continue
        }

        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }

        for _, addr := range addrs {
            ipNet, ok := addr.(*net.IPNet)
            if !ok {
                continue
            }

            // 只处理IPv4地址
            ipv4 := ipNet.IP.To4()
            if ipv4 == nil {
                continue
            }

            // 获取原始掩码字节切片
            mask := ipNet.Mask
            // 确保是IPv4掩码（4字节长度）
            if len(mask) != net.IPv4len {
                continue
            }

            // 计算广播地址
            broadcast := make(net.IP, net.IPv4len)
            for i := 0; i < net.IPv4len; i++ {
                broadcast[i] = ipv4[i] | ^mask[i]
            }
            broadcastAddrs = append(broadcastAddrs, broadcast)
        }
    }

    if len(broadcastAddrs) == 0 {
        return nil, errors.New("未找到有效网络接口")
    }

    // 创建UDP socket
    conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)
    fmt.Println("本地监听地址:", localAddr)

    message := []byte("lgcloud")

    // 发送广播到所有发现的广播地址
    for _, bcAddr := range broadcastAddrs {
        target := &net.UDPAddr{
            IP:   bcAddr,
            Port: 7678,
        }
        _, err = conn.WriteTo(message, target)
        if err != nil {
            fmt.Printf("发送到 %s 失败: %v\n", bcAddr, err)
            continue
        }
        fmt.Printf("已发送广播到 %s\n", bcAddr)
    }

    // 设置读取超时
    conn.SetDeadline(time.Now().Add(2 * time.Second))

	// 加入已分配IP池
    ipMutex.Lock()
	defer ipMutex.Unlock()

    buf := make([]byte, 1024)
    for {
        n, addr, err := conn.ReadFrom(buf)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                break // 正常超时退出
            }
            return devices, err
        }

        response := string(buf[:n])
        fmt.Printf("收到来自 %s 的响应: %s\n", addr.String(), response)
        parts := strings.Split(response, ":")
        if len(parts) >= 3 {
            ip := strings.Split(addr.String(), ":")[0]
			allocatedIPs[ip] = true
            devices = append(devices, DeviceInfo{
                IP:       ip,
                Type:     parts[0],
                ID:       parts[1],
                Name:     strings.Join(parts[2:], ":"),
                Response: response,
            })
        }
    }

    // 按IP排序
    sort.Slice(devices, func(i, j int) bool {
        return strings.Compare(devices[i].IP, devices[j].IP) < 0
    })
    return devices, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// 添加设备图标生成函数
func createDeviceIcon(name string) fyne.CanvasObject {
    initial := ""
    if len(name) > 0 {
        initial = strings.ToUpper(string(name[0]))
    }
    
    // 创建圆角矩形
    rect := canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x7B, B: 0xFF, A: 0xFF})
    rect.CornerRadius = 10  // 设置圆角半径
    
    text := canvas.NewText(initial, color.White)
    text.TextSize = 18
    text.Alignment = fyne.TextAlignCenter
    textContainer := container.NewStack(
        rect,
        container.NewCenter(text),
    )
    Icon := container.NewCenter(container.NewGridWrap(fyne.NewSize(28, 28),textContainer))
    return Icon
}

func parseBootParams(args []string) (width, height, rpaPort int) {
    // 默认值
    width = 720
    height = 1280
    rpaPort = 0

    for _, arg := range args {
        // 解析分辨率
        if strings.HasPrefix(arg, "androidboot.dobox_width=") {
            if val, err := strconv.Atoi(strings.Split(arg, "=")[1]); err == nil {
                width = val
            }
        }
        if strings.HasPrefix(arg, "androidboot.dobox_height=") {
            if val, err := strconv.Atoi(strings.Split(arg, "=")[1]); err == nil {
                height = val
            }
        }
        
        // 解析备用端口
        if strings.HasPrefix(arg, "androidboot.ro.rpa=") {
            if val, err := strconv.Atoi(strings.Split(arg, "=")[1]); err == nil {
                rpaPort = val
            }
        }
    }
    return
}

// -------------------- Docker API 模块 --------------------
// getContainers 通过访问 http://deviceIP:2375/containers/json 获取容器列表
func getContainers(deviceIP string) ([]ContainerInfo, error) {
    url := "http://" + deviceIP + ":2375/containers/json?all=true"
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var containers []ContainerInfo
    if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
        return nil, err
    }

	// 初始化设备索引
    //if _, ok := deviceIdxs[deviceIP]; !ok {
        deviceIdxs[deviceIP] = make(map[int]bool)
    //}

	imageCacheLock.RLock()
    defer imageCacheLock.RUnlock()

    // 遍历每个容器获取详细信息
    for i := range containers {
		cmdArgs := strings.Split(containers[i].Command, " ")
        w, h, rpa := parseBootParams(cmdArgs)
        containers[i].Width = w
        containers[i].Height = h
        containers[i].RpaPort = rpa
		//fmt.Println("容器初始", w, h, rpa)
		// 提取分辨率
		
		// 匹配镜像URL获取名称
        for _, img := range fullImageCache {
            if img.URL == containers[i].Image {
                containers[i].ImageName = img.Name
                break
            }
        }
		
        detailURL := fmt.Sprintf("http://%s:2375/containers/%s/json", deviceIP, containers[i].ID)
        detailResp, err := http.Get(detailURL)
        if err != nil {
            continue
        }
        
        var detail struct {
			Args        []string          `json:"Args"`
			Cmd        []string          `json:"Cmd"`
            HostConfig struct {
                Devices []struct {
					PathOnHost string `json:"PathOnHost"`
                    PathInContainer string `json:"PathInContainer"`
                } `json:"Devices"`
            } `json:"HostConfig"`
            Config struct {
                Labels map[string]string `json:"Labels"`
            } `json:"Config"`
            NetworkSettings struct {
				Networks map[string]struct {
					IPAMConfig struct {
						IPv4Address string `json:"IPv4Address"`
					} `json:"IPAMConfig"`
					IPAddress   string `json:"IPAddress"`
					Gateway     string `json:"Gateway"`
					IPPrefixLen int `json:"IPPrefixLen"`
				} `json:"Networks"`
			} `json:"NetworkSettings"`
        }
        
        if err := json.NewDecoder(detailResp.Body).Decode(&detail); err == nil {
			//fmt.Println("获取容器详细", detail)
			if containers[i].RpaPort == 0 {
				w, h, rpa = parseBootParams(detail.Args)
				containers[i].Width = w
				containers[i].Height = h
				containers[i].RpaPort = rpa
				//fmt.Println("容器详细", w, h, rpa)
            }
            // 从Labels获取idx
            if idxStr, ok := detail.Config.Labels["idx"]; ok {
                if idx, err := strconv.Atoi(idxStr); err == nil {
                    deviceIdxs[deviceIP][idx] = true
                    containers[i].Idx = idx
                }
            } else {
                // 从设备路径推断idx
                for _, dev := range detail.HostConfig.Devices {
                    if strings.Contains(dev.PathInContainer, "/dev/vndbinder") {
                        parts := strings.Split(dev.PathOnHost, "binder")
                        if len(parts) > 1 {
                            num, _ := strconv.Atoi(parts[1])
                            idx := num / 3
                            deviceIdxs[deviceIP][idx] = true
                            containers[i].Idx = idx
                            break
                        }
                    }
                }
            }
            
            // 获取网络信息做冲突IP检查
            if value, ok := detail.NetworkSettings.Networks["myt"]; ok {
				ipMutex.Lock()
				if value.IPAMConfig.IPv4Address != "" {
					allocatedIPs[value.IPAMConfig.IPv4Address] = true
				} else if value.IPAddress != "" {
					allocatedIPs[value.IPAddress] = true
				}
				ipMutex.Unlock()
			}
        }
        detailResp.Body.Close()
    }
    return containers, nil
}


// startContainer 调用 Docker API 启动容器（POST /containers/{id}/start）
func startContainer(deviceIP, containerID string) error {
	url := "http://" + deviceIP + ":2375/containers/" + containerID + "/start"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return errors.New("启动容器失败，HTTP状态码：" + strconv.Itoa(resp.StatusCode))
	}
	return nil
}

// stopContainer 调用 Docker API 停止容器（POST /containers/{id}/stop）
func stopContainer(deviceIP, containerID string) error {
	url := "http://" + deviceIP + ":2375/containers/" + containerID + "/stop"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return errors.New("停止容器失败，HTTP状态码：" + strconv.Itoa(resp.StatusCode))
	}
	return nil
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

// 修改refreshScreenshot方法
func (c *ContainerCard) safeRefreshScreenshot() {
    port := getContainerPort(c.info, 9083)
    if port == 0 || !c.active {
		if c.info.RpaPort != 0 && c.active {
			port = c.info.RpaPort
		} else {
			return
        }
        return
    }

    // 连接处理
    if c.handle <= 0 {
		handle, err := openDevice(c.deviceIP, port, 3)
		if handle <= 0 || err != nil {
			handle, err := openDevice(c.deviceIP, c.info.RpaPort, 3)
			if handle <= 0 || err != nil {
				return
			} else {
				c.handle = handle
			}
		} else {
			c.handle = handle
		}
    }

    // 获取截图
    data, err := takeScreenshot(c.handle, 0, 80)
    if err != nil {
        return
    }

    // 解码和缓存
    img, err := png.Decode(bytes.NewReader(data))
    if err != nil {
        return
    }

    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.lastImage = img
    c.img.Image = img
    c.img.Refresh()
}

// 新增队列处理逻辑
func processScreenshotQueue() {
    for {
        select {
        case card := <-screenshotQueue:
            card.safeRefreshScreenshot()
        case <-screenshotStopChan:
            return
        }
    }
}

func restartScreenshotWorkers() {
    close(screenshotStopChan)
    screenshotStopChan = make(chan struct{})
    go processScreenshotQueue()
}

// -------------------- 工具函数 --------------------
func getContainerPort(info *ContainerInfo, privatePort int) int {
	for _, p := range info.Ports {
		if p.PrivatePort == privatePort {
			return p.PublicPort
		}
	}
	return privatePort
}

// -------------------- 主题定制 --------------------
type ModernTheme struct{}

func (ModernTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 245, G: 245, B: 245, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 63, G: 81, B: 181, A: 255}
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (ModernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (ModernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (ModernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameText:
		return 14
	}
	return theme.DefaultTheme().Size(name)
}

// 修改autoRefresh方法
func (c *ContainerCard) autoRefresh() {
    ticker := time.NewTicker(time.Duration(currentInterval) * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if c.running && c.active {
                select {
                case screenshotQueue <- c:
                default: // 防止队列阻塞
                }
            }
        case <-c.stopChan:
            if c.handle > 0 {
                C.closeDevice(c.handle)
                c.handle = 0
            }
            fmt.Printf("关闭\n", c.deviceIP)
            return
        }
    }
}

// -------------------- 容器卡片实现 --------------------
func NewContainerCard(info *ContainerInfo, deviceIP string) *ContainerCard {
	c := &ContainerCard{
		info:       info,
		deviceIP:   deviceIP,
		img:        canvas.NewImageFromImage(generateDefaultImage(160, 280)),
		status:     widget.NewLabel(""),
		stopChan:   make(chan struct{}),
        running:    info.State == "running",
        active:     true,
	}
	c.ExtendBaseWidget(c)
	c.img.FillMode = canvas.ImageFillContain
	c.img.SetMinSize(fyne.NewSize(160, 280))
	go c.autoRefresh()
	return c
}

func generateDefaultImage(width, height int) image.Image {
    img := image.NewRGBA(image.Rect(0, 0, width, height))
    draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 200, 200, 255}}, image.Point{}, draw.Src)
    return img
}

func (c *ContainerCard) CreateRenderer() fyne.WidgetRenderer {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 添加默认占位图
    if c.img.Image == nil {
        c.img.Image = generateDefaultImage(160, 280)
    }

	//statusIcon := widget.NewIcon(theme.InfoIcon())
	//if c.running {
	//	statusIcon = widget.NewIcon(theme.ConfirmIcon())
	//}
	c.checkbox = widget.NewCheck("", nil)
	c.checkbox.Checked = selectedContainers[c.info.ID].ID != ""
    c.checkbox.OnChanged = func(checked bool) {
        if checked {
			//fmt.Println("获取容器列表:", c.info.ID)
            selectedContainers[c.info.ID] = *c.info
        } else {
            delete(selectedContainers, c.info.ID)
        }
        updateSelectAllState()
    }
    c.checkbox.Refresh()

	startBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		go c.startContainer()
	})
	stopBtn := widget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		go c.stopContainer()
	})

	idxLabel := widget.NewLabel(fmt.Sprintf("T%d", c.info.Idx))

	statusBar := container.NewHBox(
		c.checkbox,
		idxLabel,
		//widget.NewLabel(strings.TrimPrefix(c.info.Names[0], "/")),
		layout.NewSpacer(),
		startBtn,
		stopBtn,
	)

	return widget.NewSimpleRenderer(
		container.NewBorder(
			nil,
			statusBar,
			nil,
			nil,
			container.NewMax(
				widget.NewButton("", func() {
					startProjection(c.deviceIP, c.info)
				}),
				c.img,
			),
		),
	)
}

func createGridView() *fyne.Container {
	grid := container.NewGridWrap(
        fyne.NewSize(180, 300),
        // 添加尺寸变化监听
        layout.NewSpacer(), // 占位符确保初始布局正确
    )
    
    // 强制刷新网格布局
    grid.Layout = layout.NewGridWrapLayout(fyne.NewSize(180, 300))
    return grid
}

func (c *ContainerCard) startContainer() {
	if err := startContainer(c.deviceIP, c.info.ID); err == nil {
		c.mu.Lock()
		c.running = true
		c.mu.Unlock()
		c.Refresh()
	}
}

func (c *ContainerCard) stopContainer() {
	if err := stopContainer(c.deviceIP, c.info.ID); err == nil {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		c.Refresh()
	}
}

// -------------------- 核心功能函数 --------------------
func startProjection(deviceIP string, info *ContainerInfo) {
	port := getContainerPort(info, 9083)
	apiport := getContainerPort(info, 9082)
	//fmt.Println("获取容器CMD", info.Command)
	cmd := exec.Command("./screen.exe", "-ip", deviceIP, "-port", fmt.Sprintf("%d", port), "-name", info.Names[0], "-uploadPort", fmt.Sprintf("%d", apiport), "-containerID", info.ID, "-width", fmt.Sprintf("%d", info.Width), "-height", fmt.Sprintf("%d", info.Height), "-bakport", fmt.Sprintf("%d", info.RpaPort))
	go cmd.Run()
}

func refreshDeviceList(filter string) {
    filteredDevices = nil
    for _, d := range allDevices {
        if strings.Contains(d.IP, filter) || 
           strings.Contains(d.Name, filter) || 
           strings.Contains(d.Type, filter) {
            filteredDevices = append(filteredDevices, d)
        }
    }
    deviceList.Refresh()
}

// 添加设备详情窗口
func showDeviceDetail(device DeviceInfo) {
    detailWindow := mainApp.NewWindow("设备详情")
    content := container.NewVBox(
        container.NewHBox(
            createDeviceIcon(device.Name[:1]),
            widget.NewLabel(device.Name),
        ),
        widget.NewForm(
            widget.NewFormItem("类型:", widget.NewLabel(device.Type)),
            widget.NewFormItem("ID:", widget.NewLabel(device.ID)),
            widget.NewFormItem("IP:", widget.NewLabel(device.IP)),
        ),
    )
    detailWindow.SetContent(content)
    detailWindow.Resize(fyne.NewSize(300, 200))
    detailWindow.Show()
}

// -------------------- 主要界面组件 --------------------
func createDeviceList() fyne.CanvasObject {
    searchEntry := widget.NewEntry()
    searchEntry.SetPlaceHolder("搜索设备...")
    searchEntry.OnChanged = func(s string) {
        refreshDeviceList(s)
    }

    deviceList = widget.NewList(
        func() int { return len(filteredDevices) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                createDeviceIcon(""),
                widget.NewLabel(""),
                layout.NewSpacer(),
                widget.NewButton("详情", nil),
                widget.NewLabel(""),
				widget.NewButton("网络设置", nil),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            cont := o.(*fyne.Container)
            device := filteredDevices[i]
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[2].(*widget.Label)
            btn := cont.Objects[4].(*widget.Button)
            netBtn := cont.Objects[6].(*widget.Button)

            check.Checked = selectedDevices[device.IP].IP != ""
            check.OnChanged = func(checked bool) {
                if checked {
                    selectedDevices[device.IP] = device
                } else {
                    delete(selectedDevices, device.IP)
                }
            }
            check.Refresh()

            cont.Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*canvas.Text).Text = strings.ToUpper(device.Name[:1])
            label.SetText(device.IP + " - " + device.Name)
            btn.OnTapped = func() {
                showDeviceDetail(device)
            }
            
			netBtn.OnTapped = func() {
				showNetworkConfigDialog(device.IP)
			}
        },
    )

	selectAllCheck = widget.NewCheck("", func(checked bool) {
        selectionLock.Lock()
        defer selectionLock.Unlock()
        
        allSelected = checked
        if checked {
            for _, d := range filteredDevices {
                selectedDevices[d.IP] = d
            }
        } else {
            selectedDevices = make(map[string]DeviceInfo)
        }
        deviceList.Refresh()
    })
	
	return container.NewBorder(
        container.NewBorder(nil, nil, container.NewHBox(selectAllCheck), nil, searchEntry),
        nil, nil, nil,
        deviceList,
    )
}

func createViewTabs() *container.AppTabs {
	listView = createListView()
	gridView = createGridView()

	return container.NewAppTabs(
		container.NewTabItem("列表视图", container.NewVScroll(listView),),
		container.NewTabItem("网格视图", container.NewVScroll(gridView),),
	)
}

// 更新全选状态
func updateSelectAllState() {
    total := len(filteredDevices)
    selected := len(selectedDevices)
    
    switch {
    case selected == 0:
        selectAllCheck.Checked = false
        //selectAllCheck.Indeterminate = false
    case selected == total:
        selectAllCheck.Checked = true 
        //selectAllCheck.Indeterminate = false
    default:
        selectAllCheck.Checked = false
        //selectAllCheck.Indeterminate = true
    }
    selectAllCheck.Refresh()
}

func createListView() *widget.List {
	list := widget.NewList(
        func() int { return len(currentContainers) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                widget.NewIcon(theme.ComputerIcon()),
                widget.NewLabel(""),
                widget.NewLabel(""), // 新增镜像名称显示
                layout.NewSpacer(),
                widget.NewButton("启动", nil),
                widget.NewButton("停止", nil),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            cont := o.(*fyne.Container)
            info := currentContainers[i]
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[2].(*widget.Label)
            startBtn := cont.Objects[5].(*widget.Button)
            stopBtn := cont.Objects[6].(*widget.Button)

            check.Checked = selectedContainers[info.ID].ID != ""
            check.OnChanged = func(checked bool) {
                if checked {
					//fmt.Println("获取容器列表:", info.ID)
                    selectedContainers[info.ID] = info
                } else {
                    delete(selectedContainers, info.ID)
                }
                updateSelectAllState()
            }
            check.Refresh()
            
			adbport := getContainerPort(&info, 5555)
            label.SetText(fmt.Sprintf("%s (%s) (adb端口：%d)", info.Names[0], info.State, adbport))
            
            startBtn.OnTapped = func() {
                for _, c := range selectedContainers {
                    go startContainer(selectedDevice, c.ID)
                }
            }
            
            stopBtn.OnTapped = func() {
                for _, c := range selectedContainers {
                    go stopContainer(selectedDevice, c.ID)
                }
            }

            cont.Objects[3].(*widget.Label).SetText(info.ImageName)
        },
    )

	list.OnSelected = func(id widget.ListItemID) {
		info := currentContainers[id]
		startProjection(selectedDevice, &info)
	}

	return list
}

// 添加批量操作工具栏
func createBatchToolbar() fyne.CanvasObject {
    return container.NewHBox(
        widget.NewButtonWithIcon("批量创建", theme.DocumentCreateIcon(), func() {
            showCreateContainerDialog()
        }),
        widget.NewButtonWithIcon("批量重启", theme.MediaPlayIcon(), func() {
            for _, c := range selectedContainers {
				go func() {
					stopContainer(selectedDevice, c.ID)
					time.Sleep(2 * time.Second)
					startContainer(selectedDevice, c.ID)
				}()
			}
        }),
        widget.NewButtonWithIcon("批量关机", theme.MediaStopIcon(), func() {
            for _, c := range selectedContainers {
                go stopContainer(selectedDevice, c.ID)
            }
        }),
        widget.NewButtonWithIcon("批量投屏", theme.VisibilityIcon(), func() {
            for _, c := range selectedContainers {
                go startProjection(selectedDevice, &c)
            }
        }),
    )
}

// -------------------- 主要业务逻辑 --------------------
func refreshDevices() {
	devices, _ := discoverDevices()
	discoveredDevices = devices
	allDevices = devices
	refreshDeviceList("")
	mainWindow.Content().Refresh()
}

// 修改refreshContainers方法
func refreshContainers() {
    // 停止所有旧卡片
    for _, card := range containerCards {
		if (card.active) {
			close(card.stopChan)
		}
        card.active = false
    }

    containers, _ := getContainers(selectedDevice)
    currentContainers = containers
    containerCards = make(map[string]*ContainerCard)

    var gridItems []fyne.CanvasObject
    for i := range containers {
		if containers[i].State != "running" || containers[i].Idx == 0 {
			continue
		}
        card := NewContainerCard(&containers[i], selectedDevice)
        containerCards[containers[i].ID] = card
        gridItems = append(gridItems, card)
    }

    gridView.Objects = gridItems
    gridView.Refresh()
    listView.Refresh()
}

func updateContainers(deviceIP string) {
	containers, err := getContainers(deviceIP)
	if err != nil {
		fmt.Println("获取容器失败:", err)
		return
	}

	currentContainers = containers
	containerCards = make(map[string]*ContainerCard)

	var gridItems []fyne.CanvasObject
	for i := range containers {
		card := NewContainerCard(&containers[i], deviceIP)
		containerCards[containers[i].ID] = card
		gridItems = append(gridItems, card)
	}

	gridView.Objects = gridItems
	gridView.Refresh()
	listView.Refresh()
}

func encodeData(data map[string]interface{}) string {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return ""
    }
    encoded := base64.URLEncoding.EncodeToString(jsonData)
    return url.QueryEscape(encoded)
}

// -------------------- 网络配置相关
// IP地址重分配函数
func ReallocateIP(oldIPStr, newSubnetCIDR string, increment int) (string, error) {
    // 解析旧IP地址
    oldIP := net.ParseIP(oldIPStr)
    if oldIP == nil {
        return "", fmt.Errorf("无效的旧IP地址: %s", oldIPStr)
    }
    oldIPv4 := oldIP.To4()
    if oldIPv4 == nil {
        return "", fmt.Errorf("仅支持IPv4地址")
    }

    // 解析新子网
    _, newNet, err := net.ParseCIDR(newSubnetCIDR)
    if err != nil {
        return "", fmt.Errorf("无效的子网CIDR: %v", err)
    }
    newPrefixLen, _ := newNet.Mask.Size()

    // 生成基础新IP
    baseNewIP := generateBaseIP(oldIPv4, newNet, newPrefixLen)
    // 应用增量调整
    newIP := applyIncrement(baseNewIP, newNet, increment)
    // 冲突检测与自动调整
    return findAvailableIP(newIP, newNet)
}

// 生成基础IP（保留最大可能的主机部分）
// generateBaseIP 修复版
func generateBaseIP(oldIP net.IP, newNet *net.IPNet, newPrefixLen int) net.IP {
    // 将IP转换为大整数
    oldInt := ipToBigInt(oldIP)
    newInt := ipToBigInt(newNet.IP)

    // 计算需要保留的位数
    //oldPrefixLen := getOriginalPrefixLen(oldIP)
    retainBits := 32 - newPrefixLen

    // 创建保留掩码（保留旧IP的主机部分）
    mask := big.NewInt(1)
    mask.Lsh(mask, uint(retainBits))
    mask.Sub(mask, big.NewInt(1))

    // 保留旧IP的主机部分
    retained := oldInt.And(oldInt, mask)

    // 合并新网络前缀和保留的主机部分
    xorMask := mask.Not(mask)
    newNetworkPart := newInt.And(newInt, xorMask)
    combined := retained.Or(newNetworkPart, retained)

    // 确保结果在子网范围内
    combined.And(combined, maskToBigInt(newNet.Mask).Or(newInt, combined))
    
    return bigIntToIP(combined)
}

// 1. 修正ipToBigInt函数以支持IPMask
func maskToBigInt(mask net.IPMask) *big.Int {
    // 将4字节的IPv4掩码转换为big.Int
    if len(mask) == 4 {
        return big.NewInt(0).SetBytes([]byte(mask))
    }
    // IPv6处理（如果需要）
    return big.NewInt(0)
}

// 修复applyIncrement函数中的掩码处理
func applyIncrement(ip net.IP, subnet *net.IPNet, increment int) net.IP {
    ipInt := ipToBigInt(ip)
    subnetSize := calculateSubnetSize(subnet)
    
    // 正确获取主机掩码
    hostMask := maskToBigInt(subnet.Mask)
    hostMask = hostMask.Not(hostMask)
    
    hostPart := ipInt.And(ipInt, hostMask)
    
    // 应用增量
    newHost := big.NewInt(0).Add(hostPart, big.NewInt(int64(increment)))
    newHost.Mod(newHost, subnetSize)
    
    // 合并网络部分
    base := ipToBigInt(subnet.IP).And(ipToBigInt(subnet.IP), hostMask.Not(hostMask))
    return bigIntToIP(base.Add(base, newHost))
}

// 3. 添加缺失的函数实现
var allocatedIPs = make(map[string]bool)
var ipMutex sync.Mutex

func isIPInUse(ip string) bool {
    ipMutex.Lock()
    defer ipMutex.Unlock()
    return allocatedIPs[ip]
}

func isGatewayIP(ipStr string, subnet *net.IPNet) bool {
    // 计算网关地址（子网第一个可用地址）
    gateway := ipToBigInt(subnet.IP)
    gateway.Add(gateway, big.NewInt(1))
    return ipStr == bigIntToIP(gateway).String()
}

// 修复findAvailableIP函数（移除未使用的mask变量）
func findAvailableIP(startIP net.IP, subnet *net.IPNet) (string, error) {
    current := ipToBigInt(startIP)
    base := ipToBigInt(subnet.IP)
    subnetSize := calculateSubnetSize(subnet)

    // 确保起始地址在子网内
    subnetEnd := big.NewInt(0).Add(base, subnetSize)
    if current.Cmp(base) < 0 || current.Cmp(subnetEnd) >= 0 {
        current = base
    }

    for i := 0; i < int(subnetSize.Int64()); i++ {
        candidate := bigIntToIP(current)
        if subnet.Contains(candidate) {
            candidateStr := candidate.String()
            if !isIPInUse(candidateStr) && !isGatewayIP(candidateStr, subnet) {
                ipMutex.Lock()
                allocatedIPs[candidateStr] = true
                ipMutex.Unlock()
                return candidateStr, nil
            }
        }
        
        current.Add(current, big.NewInt(1))
        // 处理回绕
        if current.Cmp(subnetEnd) >= 0 {
            current = base
        }
    }
    return "", fmt.Errorf("未找到可用IP")
}

// 辅助函数
func ipToBigInt(ip net.IP) *big.Int {
    return big.NewInt(0).SetBytes(ip.To4())
}

func bigIntToIP(i *big.Int) net.IP {
	group1 := big.NewInt(i.Int64()).Rsh(i, 24)
	group2 := big.NewInt(i.Int64()).Rsh(i, 16)
	group3 := big.NewInt(i.Int64()).Rsh(i, 8)
	group4 := big.NewInt(i.Int64())
    return net.IPv4(
        byte(group1.Uint64()),
        byte(group2.Uint64()&0xff),
        byte(group3.Uint64()&0xff),
        byte(group4.Uint64()&0xff),
    )
}

func calculateSubnetSize(subnet *net.IPNet) *big.Int {
    ones, bits := subnet.Mask.Size()
    return big.NewInt(1).Lsh(big.NewInt(1), uint(bits-ones))
}

func getGatewayIP(subnet *net.IPNet) string {
    gateway := ipToBigInt(subnet.IP)
    gateway.Add(gateway, big.NewInt(1))
    return bigIntToIP(gateway).String()
}

func isBroadcast(ip net.IP, subnet *net.IPNet) bool {
    broadcast := ipToBigInt(subnet.IP)
    mask := ipToBigInt(net.IP(subnet.Mask))
    broadcast.Or(broadcast, mask.Not(mask))
    return ip.Equal(bigIntToIP(broadcast))
}

// 获取客户端网络配置
func getClientNetworkConfig(deviceIP string) (subnetCIDR, gateway, parentIf string, err error) {
    // 获取与设备通信的本地接口
    iface, err := findInterfaceByIP(deviceIP)
    if err != nil {
        return "", "", "", err
    }

    // 获取接口详细信息
    addrs, err := iface.Addrs()
    if err != nil || len(addrs) == 0 {
        return "", "", "", fmt.Errorf("无法获取接口地址")
    }

    // 获取网关（跨平台实现）
    gateway, err = getDefaultGateway(iface.Name)
    if err != nil {
        return "", "", "", err
    }

	var ipNet *net.IPNet
	var baseIP net.IP
	var ok bool
	for _, addr := range addrs { 
		// 解析CIDR
		ipNet, ok = addr.(*net.IPNet)
		if !ok {
			//return "", "", "", fmt.Errorf("无效的IP地址")
			continue
		}
		ipaddrCidr := strings.Split(ipNet.String(),"/")
		if len(ipaddrCidr) > 1 {
			subnetCIDR = ipaddrCidr[1]
		} else {
			subnetCIDR = "24"
		}
		
		// 生成推荐子网（当前子网+1）
		baseIP = ipNet.IP.To4()
		if baseIP == nil {
			//return "", "", "", fmt.Errorf("仅支持IPv4")
			continue
		} else {
			break
		}
    }
    if baseIP == nil {
		return "", "", "", fmt.Errorf("仅支持IPv4")
	}
    cidrLen, err := strconv.Atoi(subnetCIDR)
    if err != nil || cidrLen <= 0 || cidrLen >=32 {
		cidrLen = 24
    }
    mask := net.CIDRMask(cidrLen, 32)
    subNetIP := baseIP.Mask(mask)
    newSubnet := fmt.Sprintf("%s/%d", 
        subNetIP.String(), cidrLen)

	fmt.Println("获取IP信息", newSubnet, baseIP, subnetCIDR, gateway, addrs, ipNet, iface)
    // 自动确定父接口（通常与设备接口同名）
    parentIf = "eth0"

    return newSubnet, gateway, parentIf, nil
}

// 查找与设备IP同子网的本地接口
func findInterfaceByIP(deviceIP string) (*net.Interface, error) {
    targetIP := net.ParseIP(deviceIP)
    ifaces, _ := net.Interfaces()

    for _, iface := range ifaces {
        addrs, _ := iface.Addrs()
        for _, addr := range addrs {
            ipNet, ok := addr.(*net.IPNet)
            if ok && ipNet.Contains(targetIP) {
                return &iface, nil
            }
        }
    }
    return nil, fmt.Errorf("未找到与%s同子网的接口", deviceIP)
}

// 跨平台获取默认网关
func getDefaultGateway(ifaceName string) (string, error) {
    switch runtime.GOOS {
    case "windows":
        return getWindowsGateway(ifaceName)
    case "linux":
        return getLinuxGateway(ifaceName)
    case "darwin":
        return getMacGateway(ifaceName)
    default:
        return "", fmt.Errorf("不支持的操作系统")
    }
}

// Windows实现
func getWindowsGateway(ifaceName string) (string, error) {
    cmd := exec.Command("route", "print", "-4")
    output, _ := cmd.Output()
    lines := strings.Split(string(output), "\n")

    for _, line := range lines {
        if strings.Contains(line, "0.0.0.0") {
            fields := strings.Fields(line)
            if len(fields) >= 3 {
                return fields[2], nil
            }
        }
    }
    return "", fmt.Errorf("网关未找到1", ifaceName)
}

// Linux实现
func getLinuxGateway(ifaceName string) (string, error) {
    cmd := exec.Command("ip", "route", "show", "default")
    output, _ := cmd.Output()
    lines := strings.Split(string(output), "\n")

    for _, line := range lines {
        if strings.Contains(line, "dev "+ifaceName) {
            fields := strings.Fields(line)
            for i, f := range fields {
                if f == "via" && len(fields) > i+1 {
                    return fields[i+1], nil
                }
            }
        }
    }
    return "", fmt.Errorf("网关未找到")
}

// MacOS实现
func getMacGateway(ifaceName string) (string, error) {
    cmd := exec.Command("netstat", "-rn")
    output, _ := cmd.Output()
    //lines := strings.Split(string(output), "\n")

    for _, line := range strings.Split(string(output), "\n") {
        if strings.HasPrefix(line, "default") && 
           strings.Contains(line, ifaceName) {
            fields := strings.Fields(line)
            if len(fields) >= 2 {
                return fields[1], nil
            }
        }
    }
    return "", fmt.Errorf("网关未找到")
}

// 检查现有网络
func getExistingNetwork(deviceIP string) (string, *NetworkConfig, error) {
    resp, err := http.Get(fmt.Sprintf("http://%s:2375/networks/myt", deviceIP))
    if err != nil || resp.StatusCode != 200 {
        return "", nil, fmt.Errorf("网络不存在")
    }
    defer resp.Body.Close()

    var network struct {
        Id   string `json:"Id"`
        IPAM struct {
            Config []struct {
                Subnet  string `json:"Subnet"`
                Gateway string `json:"Gateway"`
            } `json:"Config"`
        } `json:"IPAM"`
        Options map[string]string `json:"Options"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
        return "", nil, err
    }

    if len(network.IPAM.Config) == 0 {
        return "", nil, fmt.Errorf("无效的网络配置")
    }

    cfg := &NetworkConfig{
        Subnet:    network.IPAM.Config[0].Subnet,
        Gateway:   network.IPAM.Config[0].Gateway,
        ParentIf:  network.Options["parent"],
        NetworkID: network.Id,
    }
    return network.Id, cfg, nil
}

// 获取本机网络接口信息
func getHostNetworkInfo() ([]map[string]string, error) {
    interfaces, err := net.Interfaces()
    if err != nil {
        return nil, err
    }

    var result []map[string]string
    for _, iface := range interfaces {
        addrs, err := iface.Addrs()
        if err != nil || iface.Flags&net.FlagUp == 0 {
            continue
        }

        for _, addr := range addrs {
            ipNet, ok := addr.(*net.IPNet)
            if !ok || ipNet.IP.IsLoopback() {
                continue
            }

            if ipNet.IP.To4() != nil {
                info := map[string]string{
                    "Interface":  iface.Name,
                    "IPAddress":  ipNet.IP.String(),
                    "SubnetMask": net.IP(ipNet.Mask).String(),
                    "Gateway":    "192.168.10.1",
                }
                result = append(result, info)
            }
        }
    }
    return result, nil
}

// 修改网络配置对话框
func showNetworkConfigDialog(deviceIP string) {
	// 获取设备现有myt网络配置
	exists := true
	netId, currentCfg, err := getExistingNetwork(deviceIP)
	
    if err != nil {
		exists = false
        showError(fmt.Sprintf("获取设备网络配置失败: %v", err))
		// 自动获取推荐配置
        subnet, gateway, parentIf, err := getClientNetworkConfig(deviceIP)
		// 获取设备当前网络配置
		currentCfg = &NetworkConfig{
			Subnet:    subnet,
			Gateway:   gateway,
			ParentIf:  parentIf,
		}
		if err != nil {
			showError(fmt.Sprintf("获取本机网络配置失败，需要自己配置网络: %v", err))
		}
        //return
    }
	fmt.Println("获取网络配置: %v", currentCfg, err, netId)

    // 创建表单元素
    subnetEntry := widget.NewEntry()
    gatewayEntry := widget.NewEntry()
    parentEntry := widget.NewEntry()

    // 设置当前值
    subnetEntry.SetText(currentCfg.Subnet)
    gatewayEntry.SetText(currentCfg.Gateway)
    parentEntry.SetText(currentCfg.ParentIf)

    // 创建验证函数
    validateInput := func() (NetworkConfig, error) {
        cfg := NetworkConfig{
            Subnet:    strings.TrimSpace(subnetEntry.Text),
            Gateway:   strings.TrimSpace(gatewayEntry.Text),
            ParentIf:  strings.TrimSpace(parentEntry.Text),
        }

        // 验证子网格式
        if _, _, err := net.ParseCIDR(cfg.Subnet); err != nil {
            return cfg, fmt.Errorf("子网格式错误（示例：192.168.1.0/24）")
        }

        // 验证网关IP
        if net.ParseIP(cfg.Gateway) == nil {
            return cfg, fmt.Errorf("网关地址无效")
        }

        return cfg, nil
    }

    dialog.ShowCustomConfirm("网络配置", "保存", "取消",
        container.NewVBox(
            widget.NewForm(
                widget.NewFormItem("网络接口", parentEntry),
                widget.NewFormItem("子网 (CIDR)", subnetEntry),
                widget.NewFormItem("网关", gatewayEntry),
            ),
        ),
        func(save bool) {
            if !save {
                return
            }
            
            newCfg, err := validateInput()
            if err != nil {
                showError(err.Error())
                return
            }

            go func() {
                // 检查网络是否已存在
                if exists {
                    if currentCfg.Subnet == newCfg.Subnet &&
                        currentCfg.Gateway == newCfg.Gateway &&
                        currentCfg.ParentIf == newCfg.ParentIf {
                        return // 配置未改变
                    }
                    
                    // 提示用户确认更改
                    dialog.ShowConfirm("网络已存在", 
                        "修改网络配置需要重启相关容器，是否继续？",
                        func(confirm bool) {
                            if confirm {
                                if err := reconfigureNetwork(deviceIP, newCfg, *currentCfg, exists); err != nil {
                                    showError(err.Error())
                                }
                            }
                        }, mainWindow)
                } else {
                    if err := reconfigureNetwork(deviceIP, newCfg, *currentCfg, exists); err != nil {
                        showError(err.Error())
                    }
                }
            }()
        },
        mainWindow,
    )
}

// 修改网络创建逻辑
func CreateNetwork(deviceIP string, cfg NetworkConfig) (string, error) {
    // 创建新网络
    data := map[string]interface{}{
        "Name": "myt",
        "Driver": "macvlan",
        "IPAM": map[string]interface{}{
            "Driver": "default",
            "Config": []map[string]string{
                {"Subnet": cfg.Subnet, "Gateway": cfg.Gateway},
            },
        },
        "Options": map[string]string{
            "parent": cfg.ParentIf,
        },
    }

    body, _ := json.Marshal(data)
    resp, err := http.Post(fmt.Sprintf("http://%s:2375/networks/create", deviceIP), 
        "application/json", bytes.NewReader(body))
    
    if err != nil {
        return "", fmt.Errorf("网络连接失败: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 201 {
        body, _ := ioutil.ReadAll(resp.Body)
        return "", fmt.Errorf("网络创建失败[%d]: %s", resp.StatusCode, string(body))
    }

    var result struct{ Id string `json:"Id"` }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Id, nil
}

// 重新配置网络
func reconfigureNetwork(deviceIP string, newCfg NetworkConfig, oldCfg NetworkConfig, exists bool) error {
    // 如果配置未变化则直接返回
    if exists && oldCfg.Subnet == newCfg.Subnet && 
       oldCfg.Gateway == newCfg.Gateway &&
       oldCfg.ParentIf == newCfg.ParentIf {
        return nil
    }

	infoText := widget.NewLabel("正在更改网络配置，实例将全部关掉...")
	var netDialog *widget.PopUp
    infoButtion := widget.NewButton("关闭",  func() { netDialog.Hide() })
	infoButtion.Hide()
	netDialog = widget.NewModalPopUp(
        container.NewVBox(
            infoText,
            infoButtion,
        ),
        mainWindow.Canvas(),
    )
    netDialog.Show()

    // 获取所有关联容器
    containers, containersIp, containerStates, err := getContainersUsingNetwork(deviceIP)
    if err != nil {
		netDialog.Hide()
        return fmt.Errorf("获取容器列表失败: %v", err)
    }
	
	// 需要关掉，不然断开正在运行的容器会有问题，导致假断开报MAC地址冲突
    for i, state := range containerStates {
		if state == "running" {
			stopContainer(deviceIP, containers[i])
		}
    }

    // 阶段1：断开所有容器
    for _, cid := range containers {
        if err := disconnectNetwork(deviceIP, cid); err != nil {
            fmt.Println("警告：容器 %s 断开网络失败: %v", cid[:12], err)
        }
    }

    // 阶段2：删除旧网络
    if exists && oldCfg.NetworkID != "" {
        req, _ := http.NewRequest("DELETE", 
            fmt.Sprintf("http://%s:2375/networks/%s", deviceIP, oldCfg.NetworkID), nil)
        if resp, err := http.DefaultClient.Do(req); err == nil {
            defer resp.Body.Close()
        } else {
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Println("错误！删除旧网络失败，重连旧网络", resp.StatusCode, string(body), err)
			var reconnectErrors []string
			for i, cid := range containers {
				if err := connectNetwork(deviceIP, cid, oldCfg, containersIp[i]); err != nil {
					reconnectErrors = append(reconnectErrors, 
						fmt.Sprintf("容器 %s: %v", cid[:12], err))
				}
			}
			if len(reconnectErrors) > 0 {
				netDialog.Hide()
				return fmt.Errorf("部分容器重连失败:\n%s", 
					strings.Join(reconnectErrors, "\n"))
			}
			netDialog.Hide()
			return fmt.Errorf("删除旧网络失败，重连旧网络: %v", err)
        }
    }

    // 阶段3：创建新网络
    newCfg.NetworkID = "" // 强制重新创建
    networkMutex.Lock()
    deviceNetConfigs[deviceIP] = newCfg
    networkMutex.Unlock()
    
    if _, err := CreateNetwork(deviceIP, newCfg); err != nil {
		netDialog.Hide()
        return fmt.Errorf("新网络创建失败: %v", err)
    }

    // 阶段4：重新连接容器
    var reconnectErrors []string
    for i, cid := range containers {
        if err := connectNetwork(deviceIP, cid, newCfg, containersIp[i]); err != nil {
            reconnectErrors = append(reconnectErrors, 
                fmt.Sprintf("容器 %s: %v", cid[:12], err))
        }
    }
    
    if len(reconnectErrors) > 0 {
		netDialog.Hide()
        return fmt.Errorf("部分容器重连失败:\n%s", 
            strings.Join(reconnectErrors, "\n"))
    }
    
    infoText.SetText("更改网络设置成功！\n注意：实例已全部关掉")
    infoButtion.Show()
	//netDialog.Hide()
    return nil
}

// 获取使用myt网络的容器
func getContainersUsingNetwork(deviceIP string) ([]string, []string, []string, error) {
    resp, err := http.Get(fmt.Sprintf("http://%s:2375/containers/json?all=true&filters={\"network\":[\"myt\"]}", deviceIP))
    if err != nil {
        return nil, nil, nil, err
    }
    
    var containers []struct{
		Id string `json:"Id"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAMConfig struct {
					IPv4Address string `json:"IPv4Address"`
				} `json:"IPAMConfig"`
				IPAddress   string `json:"IPAddress"`
				Gateway     string `json:"Gateway"`
				IPPrefixLen int `json:"IPPrefixLen"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
		State string `json:"State"`
	}
    json.NewDecoder(resp.Body).Decode(&containers)
    //fmt.Println("获取使用myt网络的容器信息: ", containers, resp.Body)
    var ids []string
    var ips []string
    var states []string
    for _, c := range containers {
        ids = append(ids, c.Id)
        if c.NetworkSettings.Networks["myt"].IPAMConfig.IPv4Address != "" {
			ips = append(ips, c.NetworkSettings.Networks["myt"].IPAMConfig.IPv4Address)
        } else {
			ips = append(ips, c.NetworkSettings.Networks["myt"].IPAddress)
		}
		states = append(states, c.State)
    }
    return ids, ips, states, nil
}

// 断开网络连接
func disconnectNetwork(deviceIP, containerID string) error {
	data := map[string]interface{}{
        "Container": containerID,
        "Force": true,
    }
    body, _ := json.Marshal(data)
    resp, err := http.Post(fmt.Sprintf("http://%s:2375/networks/myt/disconnect", deviceIP), "application/json", bytes.NewReader(body))
    defer resp.Body.Close()
    if err != nil {
		return nil
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("断开网络失败，状态码：%d", resp.StatusCode)
    }
    return nil
}



// 连接网络
func connectNetwork(deviceIP, containerID string, newCfg NetworkConfig, oldIP string) error {
	// 获得子网cidr长度
	subnetCIDR := "24"
	subnetCidr := strings.Split(newCfg.Subnet,"/")
	if len(subnetCidr) > 1 {
		subnetCIDR = subnetCidr[1]
	} else {
		subnetCIDR = "24"
	}
	cidrLen, err := strconv.Atoi(subnetCIDR)
    if err != nil || cidrLen <= 0 || cidrLen >=32 {
		cidrLen = 24
    }
    
    ip, err := ReallocateIP(oldIP, newCfg.Subnet, 0)
    //fmt.Println("分配新IP：", ip, err)
    
    // 填充数据
    data := map[string]interface{}{
        "Container": containerID,
        "EndpointConfig": map[string]interface{}{
			"IPAMConfig": map[string]string{
				"IPv4Address": ip, // 实现IP分配逻辑
			},
		},
		"Gateway": newCfg.Gateway,
		"IPAddress": ip,
		"IPPrefixLen": cidrLen,
    }
    body, _ := json.Marshal(data)
    resp, err := http.Post(fmt.Sprintf("http://%s:2375/networks/myt/connect", deviceIP), "application/json", bytes.NewReader(body))
    defer resp.Body.Close()
    if err != nil {
		return nil
    }
    if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("错误！连接网络失败，", resp.StatusCode, string(body), err)
        return fmt.Errorf("连接网络失败，状态码：%d", resp.StatusCode)
    }
    return nil
}

// -------------------- 创建容器相关 --------------
func isCompatibleType(deviceType, selectedType string) bool {
    switch selectedType {
    case "q1_10", "c1_10":
        return deviceType == "q1_10" || deviceType == "c1_10"
    case "q1", "C1", "":
        return deviceType == "q1" || deviceType == "C1" || deviceType == ""
    default:
        return deviceType == selectedType
    }
}

func refreshHostList(deviceType string, currentIP string) []DeviceInfo {
    tempFilteredDevices := []DeviceInfo{}

    // 总是包含当前设备
    var currentDevice DeviceInfo
    for _, d := range allDevices {
        if d.IP == currentIP {
            currentDevice = d
            break
        }
    }
    tempFilteredDevices = append(tempFilteredDevices, currentDevice)

    // 添加其他兼容设备
    for _, d := range allDevices {
        if d.IP != currentIP && isCompatibleType(d.Name, deviceType) {
            tempFilteredDevices = append(tempFilteredDevices, d)
        }
    }
    return tempFilteredDevices
}

func isValidImageFormat(image string) bool {
    // 简单验证格式：至少包含一个/和:
    parts := strings.Split(image, ":")
    if len(parts) < 2 {
        return false
    }
    return strings.Contains(parts[0], "/") || 
           strings.Contains(parts[0], ".") // 支持仓库地址或本地镜像
}

// 添加创建容器对话框
func showCreateContainerDialog() {
	if len(selectedDevice) == 0 {
		showError("请选择设备！")
		return
	}
	
	// 获取当前选中设备信息
    var currentDevice DeviceInfo
    for _, d := range allDevices {
        if d.IP == selectedDevice {
            currentDevice = d
            break
        }
    }

    // 初始化设备类型
    currentDeviceType = currentDevice.Name
    if currentDeviceType == "" {
        currentDeviceType = "C1"
    }
	
	// 异步加载镜像列表
    loadChan := make(chan bool, 1)
    go func() {
        err := fetchImageList(currentDeviceType)//currentDevice.Type
        filterImageList(currentDeviceType)
        //fyne.CurrentApp().Driver().RunInMainThread(func() {
            if err != nil {
                showError(fmt.Sprintf("获取镜像失败: %v", err))
                loadChan <- false
                return
            }
            loadChan <- true
        //})
    }()
    
    // 创建进度提示
    progress := widget.NewProgressBarInfinite()
    loadingPop := widget.NewModalPopUp(
        container.NewVBox(
            widget.NewLabel("正在加载镜像列表..."),
            progress,
        ),
        mainWindow.Canvas(),
    )
    loadingPop.Show()
    
    // 异步更新UI
    go func() {
        success := <-loadChan
        loadingPop.Hide()
        if !success {
            return
        }
		imageCacheLock.RLock()
		defer imageCacheLock.RUnlock()

		// 过滤匹配当前设备类型的镜像
		var (
			customImageEntry *widget.Entry
			imageOptions    []string
			imageMap        = make(map[string]string)
		)

		// 生成选项时添加自定义选项
		imageOptions = append(imageOptions, "自定义镜像")
		for _, img := range filteredImageCache {
			imageOptions = append(imageOptions, img.Name)
			imageMap[img.Name] = img.URL
		}

		selectEntry := widget.NewSelect(imageOptions, func(s string) {
			// 显示/隐藏自定义输入框
			if s == "自定义镜像" {
				customImageEntry.Show()
			} else {
				customImageEntry.Hide()
			}
		})
		//selectEntry.PlaceHolder = "选择镜像"
		
			// 添加自定义镜像输入框
		customImageEntry = widget.NewEntry()
		customImageEntry.PlaceHolder = "输入镜像地址（例如：registry.example.com/myimage:tag）"
		customImageEntry.Hide() // 默认隐藏

		nameEntry := widget.NewEntry()
		countEntry := widget.NewEntry()
		ipaddrEntry := widget.NewEntry()
		nameEntry.SetText("T000")
		countEntry.SetText("1")
		
		// 独立IP模式
		ipaddrEntry.SetText(currentDevice.IP)
		ipaddrEnCon := container.NewAdaptiveGrid(2,widget.NewLabel("IP地址起始"),ipaddrEntry)
		ipaddrEnCon.Hide()
		netDedicatedIP := "bridge"
		IPRadio := widget.NewRadioGroup([]string{"共享IP", "MACLVLAN独立IP"}, func(s string) {
			if s == "MACLVLAN独立IP" {
				netDedicatedIP = "myt"
				ipaddrEnCon.Show()
			} else {
				netDedicatedIP = "bridge"
				ipaddrEnCon.Hide()
			}
		})
		IPRadio.SetSelected("共享IP") // 默认选项
		ipaddrContainer := container.NewVBox(IPRadio, ipaddrEnCon)

		currentResolution := ""
		resolutionSelect := widget.NewSelect([]string{"720x1280", "1080x1920"}, func(s string) {
			if s != currentResolution {
				currentResolution = s
			}
		})
		resolutionSelect.SetSelected("720x1280")
		
		isLocal := true
		modeRadio := widget.NewRadioGroup([]string{"PC缓冲下载", "设备下载"}, func(s string) {
			if s == "PC缓冲下载" {
				isLocal = true
			} else {
				isLocal = false
			}
		})
		modeRadio.SetSelected("PC缓冲下载") // 默认选项

		Cdialog = widget.NewModalPopUp(container.NewHBox(
			container.NewVBox(
				widget.NewLabel("创建新容器"),
				//widget.NewLabel("选择目标主机:"),
				createHostCheckList(currentDevice),
				widget.NewLabel("可用镜像列表:"),
				selectEntry,
				customImageEntry,
				widget.NewLabel("镜像来源:"),
				modeRadio),
			container.NewVBox(
				layout.NewSpacer(),
				container.NewAdaptiveGrid(2,
					widget.NewLabel("容器名称前缀"),
					nameEntry,
					widget.NewLabel("创建数量"),
					countEntry,
				),
				ipaddrContainer,
				container.NewAdaptiveGrid(2,widget.NewLabel("分辨率"), resolutionSelect),
				widget.NewButton("创建", func() {
					var selectedImage string
					// 获取镜像地址
					if selectEntry.Selected == "自定义镜像" {
						selectedImage = strings.TrimSpace(customImageEntry.Text)
						if selectedImage == "" {
							showError("必须输入自定义镜像地址")
							return
						}
						if !isValidImageFormat(selectedImage) {
							showError("镜像格式无效，示例：myregistry/image:tag")
							return
						}
					} else {
						selectedImage = imageMap[selectEntry.Selected]
						if selectedImage == "" {
							showError("请选择有效镜像")
							return
						}
					}
					countC, err := strconv.Atoi(countEntry.Text)
					if err != nil || countC <= 0 || len(nameEntry.Text) == 0 || len(selectedImage) == 0 {
						showError("请选择设备及镜像，填写名称和数量")
					} else {
						go handleCreateContainers(nameEntry.Text, countC, resolutionSelect.Selected, selectedImage, netDedicatedIP, ipaddrEntry.Text, isLocal)
						Cdialog.Hide()
					}
				}),
				widget.NewButton("取消",  func() { Cdialog.Hide() }),
			)),
			mainWindow.Canvas(),
		)
		if taskWindow != nil {
			taskWindow.Show()
		} else {
			createTaskWindow()
		}
		//Cdialog.Resize(fyne.NewSize(400, 400))
		Cdialog.Show()
    }()
}

// 创建主机多选列表
func createHostCheckList(currentDevice DeviceInfo) fyne.CanvasObject {
    // 自动选中当前设备
    createSelectedDevices = map[string]DeviceInfo{currentDevice.IP: currentDevice}
	
    // 创建统计标签
    selectedCountLabel := widget.NewLabel("")
    updateSelectedCount := func() {
        count := len(createSelectedDevices)
        selectedCountLabel.SetText(fmt.Sprintf("已选设备: %d 台（包含当前设备）", count))
    }

    // 初始化设备列表
    tempFilteredDevices := refreshHostList(currentDeviceType, currentDevice.IP)

    // 设备列表组件
    hostList := widget.NewList(
        func() int { return len(tempFilteredDevices) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                widget.NewLabel(""),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            device := tempFilteredDevices[i]
            cont := o.(*fyne.Container)
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[1].(*widget.Label)

            // 自动选中当前设备且不可取消
            if device.ID == currentDevice.ID {
                check.Checked = true
                check.Disable()
            } else {
                check.Checked = createSelectedDevices[device.IP].IP != ""
                check.Enable()
            }
            check.Refresh()

            check.OnChanged = func(checked bool) {
                if checked {
                    createSelectedDevices[device.IP] = device
                } else {
                    delete(createSelectedDevices, device.IP)
                }
                updateSelectedCount()
            }

            label.SetText(fmt.Sprintf("%s (%s)", device.IP, device.Name))
        },
    )
    listcontainer := container.NewVBox(selectedCountLabel, container.NewGridWrap(fyne.NewSize(400, 150), hostList))

    updateSelectedCount()
    return listcontainer
}

// 显示详细错误对话框
func showError(msg string) {
    Edialog := dialog.NewError(fmt.Errorf(msg), mainWindow)
    Edialog.Show()
}

// 检查镜像是否存在
func checkImageExists(deviceIP string, imageName string) (bool, error) {
    resp, err := http.Get(fmt.Sprintf("http://%s:2375/images/%s/json", deviceIP, imageName))
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    return resp.StatusCode == 200, nil
}

// 创建悬浮任务队列窗口
func createTaskWindow() {
    // 展开/收起按钮
    arrowBtn := widget.NewButtonWithIcon("", theme.MenuDropUpIcon(), func() {
        if taskWindow.Content.Size().Height < 100 {
            expandTaskWindow()
        } else {
            collapseTaskWindow()
        }
    })

    // 任务列表容器
    taskListBox = container.NewVBox()
    
    scroll := container.NewVScroll(taskListBox)
    scroll.SetMinSize(fyne.NewSize(300, 200))

    // 主容器
    mainCont := container.NewBorder(
        arrowBtn,
        nil,
        nil, nil,
        scroll,
    )
    
    taskWindow = widget.NewPopUp(mainCont, mainWindow.Canvas())
    taskWindow.Move(fyne.NewPos(
        mainWindow.Canvas().Size().Width - 320,
        90,
    ))
    taskId = 0
    taskWindow.Show()
}

// 窗口展开/收起动画
func expandTaskWindow() {
    taskWindow.Resize(fyne.NewSize(300, 300))
    taskWindow.Refresh()
}

func collapseTaskWindow() {
    taskWindow.Resize(fyne.NewSize(300, 20))
    taskWindow.Refresh()
}

// 添加任务到队列并更新UI
func addTaskToQueue(taskidC int, task CreateTask) {
    queueLock.Lock()
    defer queueLock.Unlock()

    cancelChan := make(chan struct{})
    item := &TaskItem{
		TaskId:      taskidC,
        DeviceIP:    task.Device.IP,
        Status:      "waiting",
        Progress:    0,
        Cancel:      cancelChan,
        isStop:      false,
        InfoLabel:   widget.NewLabel(fmt.Sprintf("%s - 等待中", task.Device.IP)),
        ProgressBar: widget.NewProgressBar(),
    }
    
    taskList = append(taskList, item)
    updateTaskListUI()
    
    go func() {
        taskQueue <- task
    }()
}

// 更新任务列表UI
func updateTaskListUI() {
    taskListBox.RemoveAll()
    for _, item := range taskList {
        row := container.NewBorder(
            nil,
            nil,
            widget.NewIcon(iconForStatus(item.Status)),
            widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
				if !item.isStop {
					close(item.Cancel)
					item.isStop = true
				}
                item.Status = "canceled"
                updateTaskListUI()
            }),
            container.NewVBox(
                item.InfoLabel,
                item.ProgressBar,
            ),
        )
        taskListBox.Add(row)
    }
    taskListBox.Refresh()
}

func iconForStatus(status string) fyne.Resource {
    switch status {
    case "done":
        return theme.ConfirmIcon()
    case "error", "canceled":
        return theme.ErrorIcon()
    default:
        return theme.InfoIcon()
    }
}

func createAndStartContainer(idx int, ip string, id string, name string, req []byte, item *TaskItem) int {
	// 发送Docker API请求
	url := fmt.Sprintf("http://%s:2375/containers/create?name=%s_%d_%s%d", ip, id, idx, name, idx)
	resp, err := http.Post(url, "application/json", bytes.NewReader(req))
	// 处理响应
	if err != nil {
		showError(fmt.Sprintf("请求失败: %v", err))
		return -1
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 201 {
		body, _ := ioutil.ReadAll(resp.Body)
		showError(fmt.Sprintf("创建失败[%d]: %s", resp.StatusCode, string(body)))
		return -1
	}

	// 启动容器
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	startContainer(ip, result["Id"].(string))
	return 0
}


// 进度跟踪Reader
type ProgressReader struct {
    reader io.Reader
    total  int64
    read   int64
    item   *TaskItem
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
    n, err := pr.reader.Read(p)
    pr.read += int64(n)
    if pr.total > 0 {
        progress := float64(pr.read) / float64(pr.total)
        updateTaskStatus(pr.item, fmt.Sprintf("上传进度 %.1f%%", progress*100), 0.5+0.3*progress)
    }
    return n, err
}

// 拉取镜像并保存为tar文件
func pullAndSaveImage(deviceIP string, imageName string, savePath string, item *TaskItem) error {
    // 使用Docker API拉取镜像
    pullURL := fmt.Sprintf("http://%s:2375/images/create?fromImage=%s", deviceIP, url.QueryEscape(imageName))
    resp, err := http.Post(pullURL, "application/json", nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // 读取拉取进度
    decoder := json.NewDecoder(resp.Body)
    for {
        var status ImagePullStatus
        if err := decoder.Decode(&status); err != nil {
            if err == io.EOF {
                break
            }
            return err
        }
        progress := 0.0
        if status.ProgressDetail.Total > 0 {
            progress = float64(status.ProgressDetail.Current) / float64(status.ProgressDetail.Total)
        }
        updateTaskStatus(item, status.Status, 0.2+0.3*progress) // 假设拉取占30%进度
    }

    // 保存镜像为tar文件
    getURL := fmt.Sprintf("http://%s:2375/images/%s/get", deviceIP, url.QueryEscape(imageName))
    resp, err = http.Get(getURL)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
    }

    file, err := os.Create(savePath)
    if err != nil {
        return err
    }
    defer file.Close()

    _, err = io.Copy(file, resp.Body)
    return err
}

// 上传镜像到设备
func loadImageToDevice(deviceIP, imagePath string, item *TaskItem) error {
    file, err := os.Open(imagePath)
    if err != nil {
        return err
    }
    defer file.Close()

    fileInfo, _ := file.Stat()
    progressReader := &ProgressReader{
        reader: file,
        total:  fileInfo.Size(),
        item:   item,
    }

    req, err := http.NewRequest("POST", 
        fmt.Sprintf("http://%s:2375/images/load", deviceIP),
        progressReader)
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/x-tar")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("上传失败: %s", string(body))
    }
    return nil
}

// 任务处理协程
func processTaskQueue() {
    for task := range taskQueue {
        // 查找对应的任务项
        var item *TaskItem
        for _, t := range taskList {
            if t.TaskId == task.TaskId {
                item = t
                break
            }
        }
        if item == nil {
            continue
        }

        // 执行创建流程
        item.Status = "processing"
        updateTaskStatus(item, "开始处理", 0)
        //showError("开始处理")
        
		if task.Network == "myt" {
			// 检查网络是否已存在
			_, currentCfg, err := getExistingNetwork(task.Device.IP)
			if err != nil {
				// 验证配置是否匹配
				//existingSubnet := currentCfg.Subnet
				//existingGateway := currentCfg.Gateway
				//if existingSubnet != cfg.Subnet || existingGateway != cfg.Gateway {
				//	return "", fmt.Errorf("网络配置不匹配，请先修改网络设置")
				//}
				//return netId, nil
				isSelect := false
				dialog.ShowConfirm(task.Device.IP+"主机的网络未配置", 
					"需要先配置myt网络参数，是否立即配置？",
					func(confirm bool) {
						if confirm {
							showNetworkConfigDialog(task.Device.IP)
						}
						isSelect = true
					}, mainWindow)
				for !isSelect {
				}
			}
			if task.Ipaddr == task.Device.IP || task.Ipaddr == "" {
				task.Ipaddr, err = ReallocateIP(task.Ipaddr, currentCfg.Subnet, 1);
				if err != nil {
					task.Ipaddr, err = ReallocateIP(task.Device.IP, currentCfg.Subnet, 2);
				}
			}	
		}
		
        
        // 检查镜像
        exists, _ := checkImageExists(task.Device.IP, task.ImageName)
        if !exists {
			if task.UseLocalImage {
				// 镜像缓存路径
				cacheDir := "./tmp_mytos"
				os.MkdirAll(cacheDir, 0755)
				imageName := task.ImageName
				
				var found bool
				var registry string
				var pkg = imageName
				var tag string
				pkg, tag, found = strings.Cut(pkg, ":")
				if !found {
					tag = "latest"
				}
				partsOfPkg := strings.Split(pkg, "/")
				if len(partsOfPkg) >= 3 {
					registry = partsOfPkg[0]
					pkg = strings.Join(partsOfPkg[1:], "/")
				}
				
				cachePath := fmt.Sprintf("tmp_%s_%s-img.tar.gz", pkg, tag)//filepath.Join(cacheDir, "dobox_rk3588-dm-rk3588-rksdk12l_base-20241225160437-img.tar.gz") //strings.ReplaceAll(imageName, "/", "_")+".tar" // fmt.Sprintf("tmp_%s_%s", d, tag)
				fmt.Println("下载路径", cachePath)

				// 检查并拉取镜像
				if _, err := os.Stat(cachePath); os.IsNotExist(err) {
					updateTaskStatus(item, "正在拉取镜像...", 0.2)
					var client dget.Client
					//if *proxy != "" {
					//	proxyUrl, err := url.Parse(*proxy)
					//	if err != nil {
					//		logrus.Fatalln("代理地址"+*proxy+"错误", err)
					//	}
					//	logrus.Info("use http proxy ", proxyUrl.String())
					//	client.SetClient(&http.Client{
					//		Transport: &http.Transport{
					//			Proxy: http.ProxyURL(proxyUrl),
					//		},
					//	})
					//} else {
						client.SetClient(http.DefaultClient)
					//}

					var err = client.Install(3, registry, pkg, tag, "linux/amd64", false, false, "", "")
					if err != nil {
						fmt.Println("下载发生错误", err)
						continue
					}
					//if err := pullAndSaveImage(task.Device.IP, imageName, cachePath, item); err != nil {
					//	updateTaskStatus(item, "镜像拉取失败: "+err.Error(), 1)
					//	continue
					//}
				}

				// 上传镜像到设备
				updateTaskStatus(item, "正在上传镜像...", 0.5)
				if err := loadImageToDevice(task.Device.IP, cachePath, item); err != nil {
					updateTaskStatus(item, "镜像上传失败: "+err.Error(), 1)
					continue
				}
			} else {
				updateTaskStatus(item, "下载镜像", 0)
				if err := pullImageWithProgress(task.Device.IP, task.ImageName, item); err != nil {
					if err.Error() != "用户取消" {
						updateTaskStatus(item, "镜像下载失败", 1)
					} else {
						updateTaskStatus(item, "canceled", 1)
					}
					continue
				}
			}
        }

        // 创建容器
        updateTaskStatus(item, "创建容器", 0.5)
        idx := findAvailableIdx(task.Device.IP)
        if idx == -1 {
            updateTaskStatus(item, "无可用坑位", 1)
            continue
        }
		
		var req []byte
		switch task.Device.Name {
			case "q1_10", "c1_10", "q1", "C1", "":
				req = buildCreateRequest_rk3588(task.ImageName, idx, task.Resolution, task.Network, task.Ipaddr)
			case "p1":
				req = buildCreateRequest_cix(task.ImageName, idx, task.Resolution)
			default:
				updateTaskStatus(item, "不支持的型号", 1)
				continue
		}

        if err := createAndStartContainer(idx, task.Device.IP, task.Device.ID, task.Name, req, item); err != 0 {
            updateTaskStatus(item, "创建失败: ", 1)
        } else {
            updateTaskStatus(item, "创建成功", 1)
        }
    }
}

// 带进度的镜像下载
func pullImageWithProgress(deviceIP string, imageName string, item *TaskItem) error {
    // 创建一个带取消功能的 context
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel() // 确保在函数结束时释放资源
    // 使用 NewRequestWithContext 创建请求，绑定 context
    req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s:2375/images/create?fromImage=%s", deviceIP, imageName), nil)
	if err != nil {
        fmt.Println("创建请求失败：", err)
        return errors.New("创建请求失败")
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        // 如果请求被取消，则 err 可能为 context.Canceled
        fmt.Println("请求失败：", err)
        return err
    }
    
    fmt.Println("获取镜像:", imageName, resp.Body, err)
    //if err != nil {
    //    return err
    //}
    defer resp.Body.Close()

    decoder := json.NewDecoder(resp.Body)
    //var lastStatus string
    for {
        select {
        case <-item.Cancel:
			cancel()
            return errors.New("用户取消")
        default:
            var status ImagePullStatus
            if err := decoder.Decode(&status); err != nil {
                //下载镜像结束也会遇到EOF解析失败，也要认为是成功了。不能直接return err
                exists, _ := checkImageExists(deviceIP, imageName)
				//fmt.Println("解析json失败但下载成功：", err, lastStatus, exists, resp.StatusCode)
                if exists {
					return nil
				} else {
					return err
				}
            }

            // 更新进度
            progress := 0.0
            if status.ProgressDetail.Total > 0 {
                progress = float64(status.ProgressDetail.Current) / float64(status.ProgressDetail.Total)
            }
            //lastStatus = status.Status
            updateTaskStatus(item, status.Status, progress)
        }
    }
}

// 更新任务状态
func updateTaskStatus(item *TaskItem, status string, progress float64) {
    queueLock.Lock()
    defer queueLock.Unlock()

    item.Status = strings.ToLower(status)
    item.Progress = progress
    item.InfoLabel.SetText(fmt.Sprintf("%s - %s", item.DeviceIP, status))
    item.ProgressBar.SetValue(progress)
    
    //fyne.CurrentApp().Driver().RunInMainThread(func() {
        taskListBox.Refresh()
    //})
}

// 处理创建容器请求
func handleCreateContainers(name string, countN int, resolution string, imageName string, network string, ipaddr string, useLocal bool) {
	if countN <= 0 || len(name) == 0 || len(imageName) == 0 {
		showError("请选择设备及镜像，填写名称和数量")
		return
	}
	if (len(createSelectedDevices) == 0) {
		showError("请选择设备")
		return
	}

    for _, device := range createSelectedDevices {
        for i := 0; i < countN; i++ {
            task := CreateTask{
                Device:     device,
                Name:       name,
                Resolution: resolution,
                Network:    network,
				Ipaddr:     ipaddr,
                ImageName:  imageName,
                TaskId:     taskId,
                UseLocalImage: useLocal,
            }
            addTaskToQueue(taskId,task)
            taskId+=1
        }
    }
}

// 简单IP分配示例
func getIPAddress(idx int) string {
    return fmt.Sprintf("192.168.101.%d", 100+idx)
}

// 生成创建请求
func buildCreateRequest_rk3588(imageAddr string, idx int, resolution string, network string, ipaddr string) []byte {
	width := "720"
	height := "1080"
	dpi := "320"
	parts := strings.Split(resolution, "x")
    if len(parts) == 2 {
		width = parts[0]
		height = parts[1]
		if width == "1080" && height == "1920" {
			dpi = "480"
		}
    }
    req := CreateContainerRequest{
        Image: imageAddr,
        Hostname: fmt.Sprintf("host-%d", idx),
        Labels: map[string]string{"idx": strconv.Itoa(idx)},
        Cmd:    []string{"androidboot.hardware=rk30board", "androidboot.dobox_fps=24", "qemu=1", "androidboot.ro.rpa="+strconv.Itoa(7100+idx), "androidboot.dobox_width="+width, "androidboot.dobox_height="+height,
		 "androidboot.dobox_dpi="+dpi, "androidboot.dobox_dpix="+dpi, "androidboot.dobox_dpiy="+dpi},
        HostConfig: struct {
			Binds         []string          `json:"Binds"`
			PortBindings  map[string][]PortBinding `json:"PortBindings"`
			Devices       []DeviceMapping   `json:"Devices"`
			DeviceCgroupRules []string      `json:"DeviceCgroupRules"`
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
			CapAdd        []string			`json:"CapAdd"`
            SecurityOpt   []string			`json:"SecurityOpt"`
            Sysctls       map[string]string `json:"Sysctls"`
            NetworkMode   string            `json:"NetworkMode"`
		}{
            Binds: []string{
                fmt.Sprintf("/mmc/data/HASH_%d:/data", idx),
                "/dev/net/tun:/dev/tun",
                "/dev/mali0:/dev/mali0",
                "/proc/1/ns/net:/dev/mnetns",
            },
            PortBindings: map[string][]PortBinding{
                "5555/tcp":  {{HostPort: strconv.Itoa(5000 + idx)}},
                "9082/tcp":  {{HostPort: strconv.Itoa(10000 + idx*3 + 2)}},
                "9083/tcp":  {{HostPort: strconv.Itoa(11000 + idx*10)}},
                "10000/tcp": {{HostPort: strconv.Itoa(10000 + idx*3)}},
                "10001/udp": {{HostPort: strconv.Itoa(10000 + idx*3 + 1)}},
                "10006/tcp": {{HostPort: strconv.Itoa(11000 + idx*10 + 1)}},
                "10007/udp": {{HostPort: strconv.Itoa(11000 + idx*10 + 2)}},
            },
            Devices: []DeviceMapping{
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3-2), PathInContainer: "/dev/binder", CgroupPermissions: "rwm"},
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3-1), PathInContainer: "/dev/hwbinder", CgroupPermissions: "rwm"},
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3), PathInContainer: "/dev/vndbinder", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/tee0", PathInContainer: "/dev/tee0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/teepriv0", PathInContainer: "/dev/teepriv0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/crypto", PathInContainer: "/dev/crypto", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/mali0", PathInContainer: "/dev/mali0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/rga", PathInContainer: "/dev/rga", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dri", PathInContainer: "/dev/dri", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/mpp_service", PathInContainer: "/dev/mpp_service", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/fuse", PathInContainer: "/dev/fuse", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/input/event0", PathInContainer: "/dev/input/event0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/cma", PathInContainer: "/dev/dma_heap/cma", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/cma-uncached", PathInContainer: "/dev/dma_heap/cma-uncached", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system", PathInContainer: "/dev/dma_heap/system", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system-dma32", PathInContainer: "/dev/dma_heap/system-dma32", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system-uncached", PathInContainer: "/dev/dma_heap/system-uncached", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system-uncached-dma32", PathInContainer: "/dev/dma_heap/system-uncached-dma32", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/ashmem", PathInContainer: "/dev/ashmem", CgroupPermissions: "rwm"},
            },
            DeviceCgroupRules: []string{"c 10:* rmw", "b 253:* rmw", "b 7:* rmw"},
            RestartPolicy: struct{ Name string `json:"Name"` }{Name: "unless-stopped"},
            CapAdd: []string{
				"SYSLOG", "AUDIT_CONTROL", "SETGID",
                "DAC_READ_SEARCH", "SYS_ADMIN", "NET_ADMIN",
                "SYS_MODULE", "SYS_NICE", "SYS_TIME",
                "SYS_TTY_CONFIG", "NET_BROADCAST", 
                "IPC_LOCK", "SYS_RESOURCE", "SYS_PTRACE",
                "WAKE_ALARM", "BLOCK_SUSPEND", "MKNOD",
            },
            SecurityOpt: []string {"seccomp=unconfined"},
            Sysctls: map[string]string { "net.ipv4.conf.eth0.rp_filter":"2" },
        },
        ExposedPorts: map[string]PortBinding{
            "5555/tcp":  {},
            "9082/tcp":  {},
            "9083/tcp":  {},
            "10000/tcp": {},
            "10001/udp": {},
            "10006/tcp": {},
            "10007/udp": {},
        },
    }

	if network == "myt" {
		NetworkingConfig := struct{
			EndpointsConfig map[string]interface{} `json:"EndpointsConfig"`
		}{
			EndpointsConfig: map[string]interface{}{
				"myt": map[string]interface{}{
					"IPAMConfig": map[string]string{
						"IPv4Address": ipaddr, // 实现IP分配逻辑
					},
				},
			},
		}
		req.NetworkingConfig = NetworkingConfig
		req.HostConfig.NetworkMode = network
    } else {
		req.HostConfig.NetworkMode = "bridge"
    }

	fmt.Println("创建容器参数:", req)

    data, _ := json.Marshal(req)
    return data
}

func buildCreateRequest_cix(imageAddr string, idx int, resolution string) []byte {
	width := "720"
	height := "1080"
	dpi := "320"
	parts := strings.Split(resolution, "x")
    if len(parts) == 2 {
		width = parts[0]
		height = parts[1]
		if width == "1080" && height == "1920" {
			dpi = "480"
		}
    }
    req := CreateContainerRequest{
        Image: imageAddr,
        Hostname: fmt.Sprintf("host-%d", idx),
        Labels: map[string]string{"idx": strconv.Itoa(idx)},
        Cmd:    []string{"androidboot.hardware=myt_p1", "androidboot.dobox_fps=24", "androidboot.ro.rpa="+strconv.Itoa(7100+idx), "androidboot.dobox_width="+width, "androidboot.dobox_height="+height,
		 "androidboot.dobox_dpi="+dpi, "androidboot.dobox_dpix="+dpi, "androidboot.dobox_dpiy="+dpi, "androidboot.selinux=permissive", "androidboot.dobox_net_ndns=1", "androidboot.dobox_net_dns1=223.5.5.5"},
        HostConfig: struct {
			Binds         []string          `json:"Binds"`
			PortBindings  map[string][]PortBinding `json:"PortBindings"`
			Devices       []DeviceMapping   `json:"Devices"`
			DeviceCgroupRules []string      `json:"DeviceCgroupRules"`
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
			CapAdd        []string			`json:"CapAdd"`
            SecurityOpt   []string			`json:"SecurityOpt"`
            Sysctls       map[string]string `json:"Sysctls"`
            NetworkMode   string            `json:"NetworkMode"`
		}{
            Binds: []string{
                fmt.Sprintf("/var/unix/%d:/dev/unix", idx),
                "/dev/net/tun:/dev/tun",
                "/dev/mali0:/dev/mali0",
                fmt.Sprintf("/mmc/data/HASH_%d:/data", idx),
            },
            PortBindings: map[string][]PortBinding{
                "5555/tcp":  {{HostPort: strconv.Itoa(5000 + idx)}},
                "9082/tcp":  {{HostPort: strconv.Itoa(10000 + idx*3 + 2)}},
                "9083/tcp":  {{HostPort: strconv.Itoa(11000 + idx*10)}},
                "10000/tcp": {{HostPort: strconv.Itoa(10000 + idx*3)}},
                "10001/udp": {{HostPort: strconv.Itoa(10000 + idx*3 + 1)}},
                "10006/tcp": {{HostPort: strconv.Itoa(11000 + idx*10 + 1)}},
                "10007/udp": {{HostPort: strconv.Itoa(11000 + idx*10 + 2)}},
            },
            Devices: []DeviceMapping{
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3-2), PathInContainer: "/dev/binder", CgroupPermissions: "rwm"},
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3-1), PathInContainer: "/dev/hwbinder", CgroupPermissions: "rwm"},
                {PathOnHost: fmt.Sprintf("/dev/binder%d", idx*3), PathInContainer: "/dev/vndbinder", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/tee0", PathInContainer: "/dev/tee0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/teepriv0", PathInContainer: "/dev/teepriv0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/mali0", PathInContainer: "/dev/mali0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dri", PathInContainer: "/dev/dri", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/fuse", PathInContainer: "/dev/fuse", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/null", PathInContainer: "/dev/input/event0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/linux,cma", PathInContainer: "/dev/dma_heap/linux,cma", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system", PathInContainer: "/dev/dma_heap/system", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/system-uncached", PathInContainer: "/dev/dma_heap/system-uncached", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/dma_heap/dsp", PathInContainer: "/dev/dma_heap/dsp", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/ashmem", PathInContainer: "/dev/ashmem", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/video0", PathInContainer: "/dev/video0", CgroupPermissions: "rwm"},
                {PathOnHost: "/dev/video1", PathInContainer: "/dev/video1", CgroupPermissions: "rwm"},
            },
            DeviceCgroupRules: []string{"c 10:* rmw", "b 252:* rmw", "b 7:* rmw"},
            RestartPolicy: struct{ Name string `json:"Name"` }{Name: "unless-stopped"},
            CapAdd: []string{
				"SYSLOG", "AUDIT_CONTROL", "SETGID",
                "DAC_READ_SEARCH", "SYS_ADMIN", "NET_ADMIN",
                "SYS_MODULE", "SYS_NICE", "SYS_TIME",
                "SYS_TTY_CONFIG", "NET_BROADCAST", 
                "IPC_LOCK", "SYS_RESOURCE", "SYS_PTRACE",
                "WAKE_ALARM", "BLOCK_SUSPEND", "MKNOD",
            },
            SecurityOpt: []string{"seccomp=unconfined"},
        },
        ExposedPorts: map[string]PortBinding{
            "5555/tcp":  {},
            "9082/tcp":  {},
            "9083/tcp":  {},
            "10000/tcp": {},
            "10001/udp": {},
            "10006/tcp": {},
            "10007/udp": {},
        },
    }

	fmt.Println("创建容器参数:", req)

    data, _ := json.Marshal(req)
    return data
}

// 按设备查找可用idx
func findAvailableIdx(deviceIP string) int {
    // 先刷新容器列表获取最新状态
    if _, err := getContainers(deviceIP); err != nil {
        return -1
    }
    
    idxMap := deviceIdxs[deviceIP]
    for i := 1; i <= 24; i++ {
        if !idxMap[i] {
            return i
        }
    }
    return -1
}

// ---------------- 获取镜像 ----------------
func getCompatibleTypes(deviceType string) []string {
    switch deviceType {
    case "q1_10", "c1_10":
        return []string{"q1_10", "c1_10"}
    case "q1", "C1", "": // 空值处理为C1
        return []string{"q1", "C1"}
    default:
        return []string{deviceType}
    }
}

func filterImageList(deviceType string) error {
	if len(fullImageCache) < 0 {
        return fmt.Errorf("fullImageCache error len: %d", len(fullImageCache))
    }
	// 在原有代码中添加兼容性处理
    compatibleTypes := getCompatibleTypes(deviceType)
    
    var filtered []MytAndroidImage
    for _, img := range fullImageCache {
        matched := false
        // 优先检查TType2
        if len(img.TType2) > 0 {
            for _, t := range img.TType2 {
                if contains(compatibleTypes, t) {
                    filtered = append(filtered, img)
                    matched = true
                    break
                }
            }
        } else {
            if contains(compatibleTypes, img.TType) {
                filtered = append(filtered, img)
                matched = true
            }
        }
        if matched {
            // 按ID降序排序
            sort.Slice(filtered, func(i, j int) bool {
                idI, _ := strconv.Atoi(filtered[i].ID)
                idJ, _ := strconv.Atoi(filtered[j].ID)
                return idI > idJ
            })
        }
    }
    filteredImageCache = filtered
    return nil
}

func fetchImageList(deviceType string) error {
    // 缓存有效期为200秒
    if time.Since(lastImageFetch) < 200*time.Second && len(fullImageCache) > 0 {
        return nil
    }

    data := map[string]interface{}{
        "ttype": deviceType,
        "spid":  0,
        "_ts":   time.Now().Unix(),
    }
    sign := "0"//genRequestSign(data) // 实现签名生成逻辑
    data["_sign"] = sign

    resp, err := http.Post("http://api.moyunteng.com/api.php?type=get_mirror_list2&data="+encodeData(data), 
        "application/json", nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var result struct {
        Code string           `json:"code"`
        Data []MytAndroidImage `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return err
    }

    if result.Code != "200" {
        return fmt.Errorf("API error: %s", result.Code)
    }

    // 按ID降序排序
    sort.Slice(result.Data, func(i, j int) bool {
        idI, _ := strconv.Atoi(result.Data[i].ID)
        idJ, _ := strconv.Atoi(result.Data[j].ID)
        return idI > idJ
    })

    imageCacheLock.Lock()
    defer imageCacheLock.Unlock()
    fullImageCache = result.Data
    lastImageFetch = time.Now()
    return nil
}

// -------------------- 主函数和初始化 --------------------
func main() {
	mainApp = app.New()
	mainWindow = mainApp.NewWindow("边缘计算管理平台 v0.3")
	mainWindow.Resize(fyne.NewSize(1280, 720))

	// 创建界面组件
	deviceList := widget.NewList(
		func() int { return len(filteredDevices) },
		func() fyne.CanvasObject { 
			return container.NewHBox(
                createDeviceIcon(""),
                widget.NewLabel(""),
            )
        },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			//o.(*widget.Label).SetText(discoveredDevices[i].IP)
			cont := o.(*fyne.Container)
            device := filteredDevices[i]
            //cont.Objects[0].(*fyne.Container).Objects[0].(*canvas.Circle).FillColor = colorForType(device.Type)
            cont.Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*canvas.Text).Text = strings.ToUpper(device.Name[:1])
            //cont.Objects[0].(*fyne.Container).Resize(fyne.NewSize(30,30))
            //fmt.Printf("图标:%T\n", )
            cont.Objects[1].(*widget.Label).SetText(device.IP)
		},
	)

	searchEntry := widget.NewEntry()
    searchEntry.SetPlaceHolder("搜索设备...")
    searchEntry.OnChanged = func(s string) {
        refreshDeviceList(s)
        deviceList.Refresh()
    }

	viewTabs := createViewTabs()
	refreshBtn := widget.NewButtonWithIcon("刷新", theme.ViewRefreshIcon(), refreshContainers)

	// 初始化截图队列
    screenshotQueue = make(chan *ContainerCard, 100)
    go processScreenshotQueue()
    
    // 添加间隔选择器
    intervalSelect = widget.NewSelect([]string{"1", "3", "5"}, func(s string) {
        newInterval, _ := strconv.Atoi(s)
        if newInterval != currentInterval {
            currentInterval = newInterval
            restartScreenshotWorkers()
        }
    })
    intervalSelect.SetSelected("1")

	toolbar := createBatchToolbar()

	selectAllCheck = widget.NewCheck("", func(checked bool) {
        selectionLock.Lock()
        defer selectionLock.Unlock()
        
        allSelected = checked
        if checked {
            for _, d := range currentContainers {
				//fmt.Println("获取容器列表:", d.ID)
                selectedContainers[d.ID] = d
                if containerCards[d.ID] != nil {
					containerCards[d.ID].checkbox.Checked = true
					containerCards[d.ID].checkbox.Refresh()
                }
            }
        } else {
            selectedContainers = make(map[string]ContainerInfo)
            for _, d := range currentContainers {
				//fmt.Println("获取容器列表:", d.ID)
                if containerCards[d.ID] != nil {
					containerCards[d.ID].checkbox.Checked = false
					containerCards[d.ID].checkbox.Refresh()
                }
            }
        }
        gridView.Refresh()
        listView.Refresh()
    })

	taskListButton := widget.NewButton("任务列表", func() {
		if taskWindow != nil {
			taskWindow.Show()
		}
	})

    // 修改布局添加间隔选择器
    mainContent := container.NewHSplit(
        container.NewBorder(
            container.NewVBox(widget.NewLabel("设备列表"),searchEntry),
            widget.NewButton("扫描设备", refreshDevices),
            nil, nil,
            container.NewVScroll(deviceList),
        ),
        container.NewBorder(
            container.NewHBox(
				selectAllCheck,
				toolbar,
                refreshBtn,
                widget.NewLabel("刷新间隔:"),
                intervalSelect,
                layout.NewSpacer(),
                taskListButton,
            ),
            nil, nil, nil,
            viewTabs,
        ),
    )
	mainContent.SetOffset(0.2)

	mainTabs := container.NewAppTabs(
		container.NewTabItem("容器管理", mainContent),
		container.NewTabItem("设备列表", createDeviceList()),
	)

	// 事件处理
	deviceList.OnSelected = func(id widget.ListItemID) {
		selectedDevice = filteredDevices[id].IP
		refreshContainers()
		viewTabs.Items[0].Content.Refresh()
		viewTabs.Items[1].Content.Refresh()
	}

	// 初始加载
	go refreshDevices()
	go processTaskQueue()
	go fetchImageList("q1")
	
	//go func() {
    //    for {
    //        if devices, err := discoverDevices(); err == nil {
    //            allDevices = devices
    //            refreshDeviceList("")
    //        }
    //        time.Sleep(30 * time.Second) // 每30秒刷新设备列表
    //    }
    //}()
	
	mainWindow.SetContent(mainTabs)
	mainWindow.ShowAndRun()
}
