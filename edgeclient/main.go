// main.go
package main

/*
#cgo CFLAGS: -I..\libs\mytrpa\include -I../libs/mytrpa/include
#cgo LDFLAGS: -L${SRCDIR} -L../libs/mytrpa/macos-m -L../libs/mytrpa/arm -L../libs/mytrpa/ubuntu -L../libs/mytrpa/lib -L../libs/mytrpa/lib64 -L../libs/mytrpa/centos7 -lmytrpc
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
    "path/filepath"
    "regexp"
    "syscall"
	"unicode"
	"mime/multipart"

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
	IsOnline bool  // 添加在线状态字段
    LastSeen time.Time
    OriginalIP string // 初始化时记录初始IP
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
        AutoRemove    bool              `json:"AutoRemove"`
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

var (
    savedDevices    []DeviceInfo
    saveDeviceLock    sync.Mutex
)
const deviceDBFile = "devices.json"

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
	mainWindow         fyne.Window
	//maindeviceList     *widget.List
	deviceList         *widget.List
	currentView        = "list"
	viewSwitch         *widget.Select
	contentStack       *container.TabItem
	listView           *widget.List
	gridView           *fyne.Container
	//discoveredDevices  []DeviceInfo
	currentContainers  []ContainerInfo
	selectedDevice     string
	containerCards     = make(map[string]*ContainerCard)
	refreshTicker      *time.Ticker
    screenshotQueue    chan *ContainerCard // 截图任务队列
    currentInterval    = 1                 // 默认1秒
    intervalSelect     *widget.Select
    screenshotStopChan = make(chan struct{})
	allDevices         []DeviceInfo
	filteredDevices    []DeviceInfo
)

// 添加多选支持结构体
type SelectableDevice struct {
    DeviceInfo
    Selected bool
}

var (
    selectedDevices         = make(map[string]DeviceInfo)  // 选中的设备
    createSelectedDevices   = make(map[string]DeviceInfo)  // 创建时选中的设备
    selectedContainers      = make(map[string]ContainerInfo) // 选中的容器
    allSelected      bool
    selectionLock    sync.Mutex
    selectAllCheck   *widget.Check
    progressBar      *widget.ProgressBar
    progressLabel    *widget.Label
    pullDialog       *widget.PopUp
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

type TaskType int

const (
    TaskCreateContainer TaskType = iota
    TaskUploadFile
)

type TaskItem struct {
    DeviceIP    string
    Status      string // waiting | processing | done | error | canceled
    Progress    float64
    Cancel      chan struct{}
    isStop      bool
    InfoLabel   *widget.Label
    ProgressBar *widget.ProgressBar
    TaskId      int
    Idx         int
    Type        TaskType
    Title       string
    SubTitle    string
    FilePath    string // 仅上传任务需要
    ContainerID string // 仅上传任务需要
    ContApiPort int // 仅上传任务需要
    CancelChan  chan struct{}
    StartTime   time.Time
    EndTime     time.Time
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
    currentDeviceType  = "C1" // 默认值
    typeOptions        = []string{"m48", "q1_10", "q1", "p1", "C1", "c1_10", "a1"}
    runningProjections = make(map[string]*os.Process) // key: containerID
    projectionLock     sync.Mutex
)

// -------------------- 加载保存设备 --------------------
// 获取配置文件路径
func getConfigPath(conf string) string {
    // 获取用户的应用支持目录（例如：/Users/username/Library/Application Support/MyApp）
    configDir, err := os.UserConfigDir()
    if err != nil {
        panic(err)
    }
    appDir := filepath.Join(configDir, "edgeclient") // 替换为你的应用名

    // 确保目录存在
    if err := os.MkdirAll(appDir, 0777); err != nil {
        panic(err)
    }

    // 返回配置文件的完整路径
    return filepath.Join(appDir, conf)
}


// 修改保存和加载逻辑
func loadData() error {
    data, err := ioutil.ReadFile(getConfigPath("data.json"))
    if err != nil {
        return err
    }
    
    var storage struct {
        Devices []DeviceInfo `json:"devices"`
        Groups  []DeviceGroup `json:"groups"`
    }
    
    if err := json.Unmarshal(data, &storage); err != nil {
        return err
    }
    
    savedDevices = storage.Devices
    deviceGroups = storage.Groups
    return nil
}

func saveData() error {
    data := struct {
        Devices []DeviceInfo `json:"devices"`
        Groups  []DeviceGroup `json:"groups"`
    }{
        Devices: savedDevices,
        Groups:  deviceGroups,
    }
    
    bytes, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(getConfigPath("data.json"), bytes, 0644)
}

// 加载保存的设备
func loadDevices() error {
	saveDeviceLock.Lock()
    defer saveDeviceLock.Unlock()
    data, err := ioutil.ReadFile(getConfigPath(deviceDBFile))
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    if err := json.Unmarshal(data, &savedDevices); err != nil {
        return err
    }
    
    for i := range savedDevices {
        savedDevices[i].OriginalIP = savedDevices[i].IP
    }

    // 加载后立即刷新IP
    go func() {
        updatedDevices, _ := discoverDevices()
        updateDeviceIPs(updatedDevices)
    }()
    return nil
}

func updateDeviceIPs(discovered []DeviceInfo) {
    // 创建发现设备的ID索引
    discoveredMap := make(map[string]string)
    for _, d := range discovered {
        discoveredMap[d.ID] = d.IP
    }

    // 更新已保存设备的IP
    for i := range savedDevices {
        if newIP, exists := discoveredMap[savedDevices[i].ID]; exists {
            savedDevices[i].IP = newIP
            savedDevices[i].LastSeen = time.Now()
            savedDevices[i].IsOnline = true
        } else {
            savedDevices[i].IsOnline = false
        }
    }
    
    saveDevices() // 自动保存更新后的IP信息
}

// 保存设备
func saveDevices() error {
    //saveDeviceLock.Lock()
    //defer saveDeviceLock.Unlock()
    data, err := json.MarshalIndent(savedDevices, "", "  ")
    if err != nil {
        return err
    }
    err = ioutil.WriteFile(getConfigPath(deviceDBFile), data, 0644)
    
    // 新增：维护分组数据
    initGroups()
    
    return err
}

func createAddDeviceDialog() {
	loadingPop := widget.NewModalPopUp(
        container.NewVBox(
            widget.NewLabel("扫描设备中..."),
        ),
        mainWindow.Canvas(),
    )
    loadingPop.Show()

    go func() {
        devices, _ := discoverDevices()
        loadingPop.Hide()

		// 创建基于ID的索引
        existingDevices := make(map[string]bool)
        saveDeviceLock.Lock()
        for _, d := range savedDevices {
            existingDevices[d.ID] = true
        }
        saveDeviceLock.Unlock()

        // 过滤已存在设备
        var newDevices []DeviceInfo
        for _, d := range devices {
            if !existingDevices[d.ID] {
                newDevices = append(newDevices, d)
            }
        }

		
        fmt.Println("扫描设备列表:", newDevices)

        // 使用新的数据结构维护选中状态
        //var selectedDevices []DeviceInfo
        type selectableDevice struct {
            DeviceInfo
            selected bool
        }
        searchFilteredDevices := newDevices
        selectDevices := make(map[string]DeviceInfo)

        list := widget.NewList(
            func() int { return len(searchFilteredDevices) },
            func() fyne.CanvasObject {
                return container.NewHBox(
                    widget.NewCheck("", nil),
                    widget.NewLabel(""),
                    widget.NewLabel(""),
                )
            },
            func(i widget.ListItemID, o fyne.CanvasObject) {
                device := searchFilteredDevices[i]
                cont := o.(*fyne.Container)
                check := cont.Objects[0].(*widget.Check)
                ipLabel := cont.Objects[1].(*widget.Label)
                nameLabel := cont.Objects[2].(*widget.Label)

                check.Checked = selectDevices[device.ID].ID != ""
                check.Refresh()
                check.OnChanged = func(selected bool) {
                    //selectDevices = append(selectDevices, d)
                    if selected {
						//fmt.Println("获取容器列表:", info.ID)
						selectDevices[device.ID] = device
					} else {
						delete(selectDevices, device.ID)
					}
                }
                ipLabel.SetText(device.IP)
                nameLabel.SetText(device.Name)
            },
        )
        
        searchEntry := widget.NewEntry()
		searchEntry.SetPlaceHolder("搜索设备...")
		searchEntry.OnChanged = func(filter string) {
			searchFilteredDevices = nil
			for _, d := range newDevices {
				if strings.Contains(d.IP, filter) || 
				   strings.Contains(d.Name, filter) || 
				   strings.Contains(d.Type, filter) {
					searchFilteredDevices = append(searchFilteredDevices, d)
				}
			}
			list.Refresh()
		}
        
        selectAllBtn := widget.NewCheck("全选", func(checked bool) {
			for _, d := range searchFilteredDevices {
				selectDevices[d.ID] = d
			}
			list.Refresh()
		})

        dialog.ShowCustomConfirm("添加设备", "添加", "取消",
			container.NewVBox(
				searchEntry,
				selectAllBtn,
				container.NewGridWrap(fyne.NewSize(400, 150), list),
			),
            func(add bool) {
                if add {
                    for _, d := range selectDevices {
                        //if d.selected {
                            exists := false
                            for _, saved := range savedDevices {
                                if saved.IP == d.IP {
                                    exists = true
                                    break
                                }
                            }
                            if !exists {
                                savedDevices = append(savedDevices, d)
                            }
                        //}
                    }
                    saveDevices()
                    refreshDeviceList("")
                    refreshDevices()
                }
            }, mainWindow)
    }()
}

func startStatusChecker() {
    go func() {
        for {
			saveDeviceLock.Lock()
            for i := range savedDevices {
                savedDevices[i].IsOnline = checkDeviceStatus(&savedDevices[i])
            }
			saveDeviceLock.Unlock()
            time.Sleep(30 * time.Second)
            deviceList.Refresh()
        }
    }()
}

func checkDeviceStatus(device *DeviceInfo) bool {
    conn, err := net.DialTimeout("tcp", device.IP+":2375", 2*time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}

// -------------------- UDP 发现模块 --------------------
// discoverDevices 使用 UDP 广播 "lgcloud" 到 7678 端口，并收集响应中带 ":" 的设备 IP
func discoverDevices() ([]DeviceInfo, error) {
    //var devices []DeviceInfo
    deviceMap := make(map[string]DeviceInfo) // 使用ID作为唯一键

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
            return []DeviceInfo{}, err
        }

        response := string(buf[:n])
        fmt.Printf("收到来自 %s 的响应: %s\n", addr.String(), response)
        parts := strings.Split(response, ":")
        if len(parts) >= 3 {
            ip := strings.Split(addr.String(), ":")[0]
            deviceID := parts[1]
			allocatedIPs[ip] = true
            // 使用ID作为唯一标识
			if _, exists := deviceMap[deviceID]; !exists {
				deviceMap[deviceID] = DeviceInfo{
					IP:       ip,
					Type:     parts[0],
					ID:       deviceID,
					Name:     strings.Join(parts[2:], ":"),
					LastSeen: time.Now(),
				}
			} else {
				// 更新IP和最后发现时间
				existing := deviceMap[deviceID]
				existing.IP = ip
				existing.LastSeen = time.Now()
				deviceMap[deviceID] = existing
			}
        }
    }
    
    // 转换map为slice
    devices := make([]DeviceInfo, 0, len(deviceMap))
    for _, d := range deviceMap {
        devices = append(devices, d)
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
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != 304 {
		return errors.New("停止容器失败，HTTP状态码：" + strconv.Itoa(resp.StatusCode))
	}
	return nil
}

// 在容器删除相关部分添加以下代码

// 增强的容器删除函数
func safeDeleteContainer(deviceIP, containerID string, idx int) error {
    // 步骤1: 停止容器
    if err := stopContainer(deviceIP, containerID); err != nil {
        return fmt.Errorf("停止容器失败: %v", err)
    }

    // 步骤2: 准备清理路径
    bindPath := fmt.Sprintf("/mmc/data/HASH_%d", idx)
    // 新增路径规范化处理
    bindPath = filepath.ToSlash(bindPath)
    parentDir := filepath.ToSlash(filepath.Dir(bindPath))
    targetDir := filepath.Base(bindPath)

    // 步骤3: 创建清理容器
    cleanupID, err := createCleanupContainer(deviceIP, parentDir, targetDir)
    if err != nil {
        return fmt.Errorf("创建清理容器失败: %v", err)
    }

    // 步骤4: 启动并等待清理完成
    if err := runAndWaitCleanup(deviceIP, cleanupID); err != nil {
        return fmt.Errorf("数据清理失败: %v", err)
    }

    // 步骤5: 删除原容器
    if err := forceDeleteContainer(deviceIP, containerID); err != nil {
        return fmt.Errorf("删除容器失败: %v", err)
    }

    return nil
}

// 创建清理容器
func createCleanupContainer(deviceIP, parentDir, targetDir string) (string, error) {
	cleanParent := filepath.ToSlash(parentDir) // 统一使用正斜杠
    
    // 验证路径格式
    if !isValidDockerPath(cleanParent) {
        return "", fmt.Errorf("无效的Docker路径: %s", cleanParent)
    }
    
    ImageName := "registry.cn-hangzhou.aliyuncs.com/whsyf/dobox:alpine"
    exists, _ := checkImageExists(deviceIP, ImageName)
    if !exists {
		pullURL := fmt.Sprintf("http://%s:2375/images/create?fromImage=%s", deviceIP, url.QueryEscape(ImageName))
		resp, err := http.Post(pullURL, "application/json", nil)
		if err != nil {
			return "", err
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
				return "", err
			}
		}
	}
	
    req := CreateContainerRequest{
        Image: ImageName,
        Cmd:   []string{"sh", "-c", fmt.Sprintf("rm -rf /cleanup/%s && test ! -d /cleanup/%s", targetDir, targetDir)},
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
			AutoRemove    bool              `json:"AutoRemove"`
        }{
            Binds: []string{fmt.Sprintf("%s:/cleanup", cleanParent)},
            AutoRemove: true, // 自动删除清理容器
        },
    }

    body, _ := json.Marshal(req)
    resp, err := http.Post(
        fmt.Sprintf("http://%s:2375/containers/create", deviceIP),
        "application/json",
        bytes.NewReader(body),
    )
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 201 {
        body, _ := ioutil.ReadAll(resp.Body)
        return "", fmt.Errorf("API错误(%d): %s", resp.StatusCode, string(body))
    }

    var result struct{ Id string `json:"Id"` }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    return result.Id, nil
}

// 添加路径验证函数
func isValidDockerPath(path string) bool {
    // 验证绝对路径格式
    if !strings.HasPrefix(path, "/") && !strings.Contains(path, ":") {
        return false
    }
    
    // 验证Docker允许的字符
    validChars := regexp.MustCompile(`^[a-zA-Z0-9/\_.:-]+$`)
    return validChars.MatchString(path)
}

// 运行并等待清理完成
func runAndWaitCleanup(deviceIP, containerID string) error {
    // 启动容器
    url := fmt.Sprintf("http://%s:2375/containers/%s/start", deviceIP, containerID)
    resp, err := http.Post(url, "application/json", nil)
    if err != nil {
        return err
    }
    resp.Body.Close()

    // 等待完成
    start := time.Now()
    for time.Since(start) < 30*time.Second {
        resp, err := http.Get(fmt.Sprintf("http://%s:2375/containers/%s/json", deviceIP, containerID))
        if err != nil {
            return err
        }
        
        var state struct {
            State struct {
                Running    bool `json:"Running"`
                ExitCode   int  `json:"ExitCode"`
            } `json:"State"`
        }
        json.NewDecoder(resp.Body).Decode(&state)
        resp.Body.Close()

        if !state.State.Running {
            if state.State.ExitCode == 0 {
                return nil
            }
            return fmt.Errorf("清理容器异常退出，状态码: %d", state.State.ExitCode)
        }
        time.Sleep(500 * time.Millisecond)
    }
    return errors.New("清理操作超时")
}

// 强制删除容器
func forceDeleteContainer(deviceIP, containerID string) error {
    req, _ := http.NewRequest(
        "DELETE",
        fmt.Sprintf("http://%s:2375/containers/%s?force=true", deviceIP, containerID),
        nil,
    )
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 204 {
        return fmt.Errorf("删除失败，状态码: %d", resp.StatusCode)
    }
    return nil
}

// 修改批量删除逻辑
func deleteSelectedContainers(deviceIP string) {
    selected := selectedContainers
    
	selectedContainers = make(map[string]ContainerInfo)
    if len(selected) == 0 {
        dialog.ShowInformation("提示", "请先选择要删除的容器", mainWindow)
        return
    }

    dialog.ShowConfirm("确认删除", 
        fmt.Sprintf("即将删除 %d 个容器及其数据，此操作不可逆！", len(selected)),
        func(ok bool) {
            if !ok {
                return
            }

            progress := widget.NewProgressBar()
            progressDialog := dialog.NewCustom("正在删除", "取消", progress, mainWindow)
            progressDialog.Show()

            go func() {
                defer progressDialog.Hide()
                total := len(selected)
                successCount := 0
                errorMessages := make([]string, 0)

                for i, c := range selected {
					fmt.Printf("删除 第 %s 个\n", i)
                    //progress.SetValue(float64(i) / float64(total))
                    
                    err := safeDeleteContainer(deviceIP, c.ID, c.Idx)
                    if err != nil {
                        errorMessages = append(errorMessages, 
                            fmt.Sprintf("容器 %s 删除失败: %v", c.Names[0], err))
                    } else {
                        successCount++
                    }
                }

                showDeleteResult(successCount, total, errorMessages)
                refreshContainers()
            }()
        }, mainWindow)
}

// 显示删除结果
func showDeleteResult(success, total int, errors []string) {
    msg := fmt.Sprintf("成功删除 %d/%d 个容器", success, total)
    if len(errors) > 0 {
        msg += "\n\n错误列表:\n" + strings.Join(errors, "\n")
    }
    
    fmt.Println("获取容器失败:", msg)

    dialog.ShowCustom("删除结果", "确定", 
        container.NewVScroll(widget.NewLabel(msg)),
        mainWindow)
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

var currentFileList *widget.List
var selectedFiles = make(map[string]bool)


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

func uploadFile(item *TaskItem, ip string, port int, filePath string) error {
    // 打开文件
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()

	fileInfo, _ := os.Stat(filePath)
    progressReader := &ProgressReader{
        reader: file,
        total:  fileInfo.Size(),
        item: item,
    }

    // 创建multipart请求
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, err := writer.CreateFormFile("file", filepath.Base(filePath))
    if err != nil {
        return err
    }
    io.Copy(part, progressReader)

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

func handleAPKInstall(item *TaskItem, ip string, uploadPort int, containerID string, 
    dockerPort int, localPath string) error {
    
    // 1. 上传文件
    if err := uploadFile(item, ip, uploadPort, localPath); err != nil {
        return fmt.Errorf("上传失败: %v", err)
    }
    
    updateTaskStatus(item, "安装中", 0.8)

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

func openSharedFolder() {
	SharedDirPath := getConfigPath("shared")
	fmt.Println("共享目录", SharedDirPath)
    // 确保文件夹存在
    if _, err := os.Stat(SharedDirPath); os.IsNotExist(err) {
        if err := os.MkdirAll(SharedDirPath, 0777); err != nil {
            dialog.ShowError(fmt.Errorf("创建共享目录失败: %v", err), mainWindow)
            return
        }
    }

    // 跨平台打开文件夹
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "windows":
        cmd = exec.Command("explorer", SharedDirPath)
    case "darwin":
        cmd = exec.Command("open", SharedDirPath)
    default: // linux
        cmd = exec.Command("xdg-open", SharedDirPath)
    }

    if err := cmd.Start(); err != nil {
        dialog.ShowError(fmt.Errorf("打开文件夹失败: %v", err), mainWindow)
    }
}

func showBatchUploadDialog() {
    // 设置共享目录路径（示例路径，可根据需要修改）
    sharedDir := getConfigPath("shared") // 客户端本机目录

    // 检查目录是否存在
    if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
        //dialog.ShowInformation("提示", "共享目录不存在，请先创建"+sharedDir, mainWindow)
        //return
        os.MkdirAll(sharedDir, 0777)
    }

    // 读取本地目录
    files, err := os.ReadDir(sharedDir)
    if err != nil {
        dialog.ShowError(fmt.Errorf("读取目录失败: %v", err), mainWindow)
        return
    }

    // 显示文件选择对话框
    showFileSelectionDialog(sharedDir, files)
}

func parseFileList(output string) []string {
    var files []string
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" && !strings.HasSuffix(line, "/") {
            files = append(files, line)
        }
    }
    return files
}

func showFileSelectionDialog(basePath string, entries []os.DirEntry) {
    var fileList []string
    selectedFiles = make(map[string]bool)

    for _, entry := range entries {
        if !entry.IsDir() {
            fileList = append(fileList, filepath.Join(basePath, entry.Name()))
        }
    }
    

    currentFileList = widget.NewList(
        func() int { return len(fileList) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                widget.NewIcon(theme.FileIcon()),
                widget.NewLabel(""),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            cont := o.(*fyne.Container)
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[2].(*widget.Label)

            filePath := fileList[i]
            label.SetText(filepath.Base(filePath))
            check.Checked = selectedFiles[filePath]
            check.OnChanged = func(b bool) {
                selectedFiles[filePath] = b
            }
        },
    )
    
    // 创建顶部工具栏
    toolbar := container.NewHBox(
        widget.NewButtonWithIcon("打开共享文件夹", theme.FolderOpenIcon(), func() {
            openSharedFolder()
        }),
        widget.NewButtonWithIcon("刷新列表", theme.ViewRefreshIcon(), func() {
            fileList = []string{}
            files, err := os.ReadDir(basePath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("读取目录失败: %v", err), mainWindow)
				return
			}
			for _, entry := range files {
				if !entry.IsDir() {
					fileList = append(fileList, filepath.Join(basePath, entry.Name()))
				}
			}
			currentFileList.Refresh()
        }),
    )
    
    // 创建带工具栏的容器
    content := container.NewBorder(
        toolbar,
        nil, nil, nil,
        container.NewGridWrap(fyne.NewSize(400, 250),currentFileList),
    )

    dialog.ShowCustomConfirm("选择要上传的文件", "开始上传", "取消",
        content,
        func(confirm bool) {
            if confirm {
				if taskWindow != nil {
					taskWindow.Show()
				} else {
					createTaskWindow()
				}
                processBatchUpload()
            }
        }, mainWindow)
}

func processBatchUpload() {
    if len(selectedFiles) == 0 {
        dialog.ShowInformation("提示", "请选择要上传的文件", mainWindow)
        return
    }

    progress := widget.NewProgressBar()
    progressDialog := dialog.NewCustom("上传进度", "取消", progress, mainWindow)
    progressDialog.Show()

    go func() {
        total := len(selectedFiles)
        current := 0
        //successCount := 0
        
        for filePath := range selectedFiles {
            current++
            progress.SetValue(float64(current)/float64(total))

            for containerID := range selectedContainers {
				//taskID := generateTaskID()
				container := selectedContainers[containerID]
				task := CreateTask{
					TaskId:     taskId,
				}
				item := TaskItem{
					TaskId:      taskId,
					Type:        TaskUploadFile,
					Status:      "waiting",
					Progress:    0,
					Title:       "文件上传",
					SubTitle:    filepath.Base(filePath),
					DeviceIP:    selectedDevice,
					FilePath:    filePath,
					ContainerID: containerID,
					Idx:         container.Idx,
					ContApiPort: getContainerPort(&container, 9082),
					CancelChan:  make(chan struct{}),
					StartTime:   time.Now(),
					InfoLabel:   widget.NewLabel(fmt.Sprintf("%s - 等待中", task.Device.IP)),
					ProgressBar: widget.NewProgressBar(),
				}
				
				queueLock.Lock()
				taskList = append(taskList, &item)
				taskId+=1
				queueLock.Unlock()
				
				taskQueue <- task
				updateTaskListUI()
            }
        }

        progressDialog.Hide()
        //showUploadResult(successCount, total*len(selectedContainers))
        //clearSelection()
    }()
}

func handleUploadTask(task CreateTask, item *TaskItem) {
	updateTaskStatus(item, "processing", 0)
	// 获取容器信息
	var err error
	// 根据文件类型选择处理方式
	if strings.HasSuffix(strings.ToLower(item.FilePath), ".apk") {
		err = handleAPKInstall(
			item,
			item.DeviceIP,
			item.ContApiPort,
			item.ContainerID,
			2375,
			item.FilePath,
		)
	} else {
		err = uploadFileToSharedDir(
			item,
			item.DeviceIP,
			item.ContApiPort,
			item.FilePath,
			"/sdcard/upload/"+filepath.Base(item.FilePath),
		)
	}
	
	if err != nil {
		fmt.Println("[%s] 上传失败: %v", item.ContainerID, err)
		updateTaskStatus(item, "error", 1)
	} else {
		//successCount++
		updateTaskStatus(item, "done", 1)
	}
}

// 新增通用上传函数
func uploadFileToSharedDir(item *TaskItem, ip string, port int, localPath, remotePath string) error {
    // 复用现有上传逻辑
    if err := uploadFile(item, ip, port, localPath); err != nil {
        return fmt.Errorf("文件传输失败: %v", err)
    }

    // 记录上传日志
    fmt.Println("文件已上传至 %s:%s", ip, remotePath)
    return nil
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
					projectionLock.Lock()
					defer projectionLock.Unlock()
					
					if _, exists := runningProjections[c.info.ID]; exists {
						dialog.ShowInformation("提示", "请勿重复打开投屏", mainWindow)
						return
					}
					
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
// 投屏startProjection函数
func startProjection(deviceIP string, info *ContainerInfo) {
    //projectionLock.Lock()
    //defer projectionLock.Unlock()

    // 检查是否已有投屏进程
    if proc, exists := runningProjections[info.ID]; exists {
        // 检查进程是否真的在运行
        if isProcessRunning(proc) {
            dialog.ShowInformation("提示", "该容器已在投屏中", mainWindow)
            return
        }
        // 进程已结束则清理记录
        delete(runningProjections, info.ID)
    }

    // 获取端口信息
    port := getContainerPort(info, 9083)
    if port == 0 {
        if info.RpaPort == 0 {
            dialog.ShowInformation("提示", "未找到可用投屏端口", mainWindow)
            return
        }
        port = info.RpaPort
    }

    // 准备启动参数
    args := []string{
        "-ip", deviceIP,
        "-port", strconv.Itoa(port),
        "-name", info.Names[0],
        "-uploadPort", strconv.Itoa(getContainerPort(info, 9082)),
        "-containerID", info.ID,
        "-width", strconv.Itoa(info.Width),
        "-height", strconv.Itoa(info.Height),
        "-bakport", strconv.Itoa(info.RpaPort),
    }

	screenexe := "./screen"
	switch runtime.GOOS {
    case "windows":
        screenexe = "./screen.exe"
    case "darwin":
		exePath, err := os.Executable()
		if err != nil {
			dialog.ShowError(fmt.Errorf("启动投屏失败: %v", err), mainWindow)
		}
		// 获取 MacOS 目录路径
		macosDir := filepath.Dir(exePath)
		// 构建 screen 的绝对路径
		screenexe = filepath.Join(macosDir, "screen")
    default:
        screenexe = "./screen"
    }
    fmt.Println("启动 ./screen ", screenexe, args)
    // 启动进程
    cmd := exec.Command(screenexe, args...)
    if err := cmd.Start(); err != nil {
        dialog.ShowError(fmt.Errorf("启动投屏失败: %v", err), mainWindow)
        return
    }

    // 记录进程并启动监控
    runningProjections[info.ID] = cmd.Process
    go monitorProjectionProcess(info.ID, cmd)
}

// 进程监控协程
func monitorProjectionProcess(containerID string, cmd *exec.Cmd) {
    err := cmd.Wait()
    projectionLock.Lock()
    defer projectionLock.Unlock()
    
    // 清理进程记录
    if _, exists := runningProjections[containerID]; exists {
        delete(runningProjections, containerID)
    }
    
    // 处理异常退出
    if err != nil {
        fmt.Printf("投屏进程异常退出: %v\n", err)
    }
}

// 检查进程是否运行
func isProcessRunning(proc *os.Process) bool {
    // Windows系统实现
    process, err := os.FindProcess(proc.Pid)
    if err != nil {
        return false
    }
    
    // 发送0信号检测进程状态
    err = process.Signal(syscall.Signal(0))
    return err == nil
}

func refreshDeviceList(filter string) {
    filteredDevices = nil
    for _, d := range savedDevices {
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
    
    // 添加删除选中设备按钮
    deleteBtn := widget.NewButtonWithIcon("删除选中设备", theme.DeleteIcon(), func() {
        if len(selectedDevices) == 0 {
            dialog.ShowInformation("提示", "请先选择要删除的设备", mainWindow)
            return
        }

        dialog.ShowConfirm("确认删除", "确定要删除选中的设备吗？", func(confirm bool) {
            if confirm {
                // 保留未选中的设备
                var newDevices []DeviceInfo
                for _, d := range savedDevices {
                    if selectedDevices[d.ID].ID == ""  {
						fmt.Println("检测删除:", d.ID, selectedDevices[d.ID].ID)
                        newDevices = append(newDevices, d)
                    }
                }
                savedDevices = newDevices
				fmt.Println("删除设备:", newDevices)
                saveDevices()
                refreshDeviceList("")
            }
        }, mainWindow)
    })

    deviceList = widget.NewList(
        func() int { return len(filteredDevices) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                createDeviceIcon(""),
                widget.NewLabel(""),
                widget.NewLabel("ID:"),
                container.NewGridWrap(fyne.NewSize(300, 36),widget.NewEntry()),
                layout.NewSpacer(),
				widget.NewButton("网络设置", nil),
                widget.NewButton("详情", nil),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            cont := o.(*fyne.Container)
            device := filteredDevices[i]
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[2].(*widget.Label)
            IDEntry := cont.Objects[4].(*fyne.Container).Objects[0].(*widget.Entry)
            netBtn := cont.Objects[6].(*widget.Button)
            btn := cont.Objects[7].(*widget.Button)

            check.Checked = selectedDevices[device.ID].ID != ""
            check.OnChanged = func(checked bool) {
                if checked {
                    selectedDevices[device.ID] = device
                } else {
                    delete(selectedDevices, device.ID)
                }
            }
            check.Refresh()
            // 设置在线状态显示
            if device.IsOnline {
                cont.Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*canvas.Rectangle).FillColor = color.NRGBA{R: 0x00, G: 0x7B, B: 0xFF, A: 0xFF}
            } else {
                cont.Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*canvas.Rectangle).FillColor = color.NRGBA{R: 0x00, G: 0x7B, B: 0xFF, A: 0x9F}
            }

            cont.Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*canvas.Text).Text = strings.ToUpper(device.Name[:1])
            label.SetText(device.IP + " - " + device.Name)
            btn.OnTapped = func() {
                showDeviceDetail(device)
            }
            
			netBtn.OnTapped = func() {
				showNetworkConfigDialog(device.IP)
			}
			
			IDEntry.SetText(device.ID)
			IDEntry.MultiLine = true
			IDEntry.Disabled()
        },
    )

	selectAllCheck = widget.NewCheck("", func(checked bool) {
        selectionLock.Lock()
        defer selectionLock.Unlock()
        
        allSelected = checked
        if checked {
            for _, d := range filteredDevices {
                selectedDevices[d.ID] = d
            }
        } else {
            selectedDevices = make(map[string]DeviceInfo)
        }
        deviceList.Refresh()
    })
	
	refreshDeviceList("")
	
	return container.NewBorder(
        container.NewBorder(nil, nil, container.NewHBox(selectAllCheck), deleteBtn, searchEntry),
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
		projectionLock.Lock()
        defer projectionLock.Unlock()
        
        if _, exists := runningProjections[info.ID]; exists {
            dialog.ShowInformation("提示", "请勿重复打开投屏", mainWindow)
            return
        }
        
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
        widget.NewButtonWithIcon("批量投屏", theme.VisibilityIcon(), func() {
            for _, c := range selectedContainers {
				projectionLock.Lock()
				
				if _, exists := runningProjections[c.ID]; exists {
					dialog.ShowInformation("提示", "请勿重复打开投屏", mainWindow)
					return
				}
				
                startProjection(selectedDevice, &c)
				projectionLock.Unlock()
            }
        }),
        widget.NewButtonWithIcon("批量上传", theme.UploadIcon(), func() {
			showBatchUploadDialog()
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
        widget.NewButtonWithIcon("批量删除", theme.DeleteIcon(), func() {
            deleteSelectedContainers(selectedDevice)
        }),
    )
}

// -------------------- 主要业务逻辑 --------------------
func refreshDevices() {
	go func() {
        devices, _ := discoverDevices()
        saveDeviceLock.Lock()
        updateDeviceIPs(devices)
		saveDeviceLock.Unlock()
        mainWindow.Content().Refresh()
		groupedListData = buildNestedListData("")
		groupedList.Refresh()
    }()
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
    for _, d := range savedDevices {
        if d.IP == currentIP {
            currentDevice = d
            break
        }
    }
    tempFilteredDevices = append(tempFilteredDevices, currentDevice)

    // 添加其他兼容设备
    for _, d := range savedDevices {
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
    for _, d := range savedDevices {
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
		Type:        TaskCreateContainer,
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
            widget.NewIcon(iconForStatus(item.Type,item.Status)),
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

func iconForStatus(Type TaskType, status string) fyne.Resource {
    switch status {
    case "done":
        return theme.ConfirmIcon()
    case "error", "canceled":
        return theme.ErrorIcon()
    default:
		if (Type == TaskUploadFile) {
			return theme.UploadIcon()
		} else {
			return theme.ComputerIcon()
		}
        //return theme.InfoIcon()
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

func handleCreateTask(task CreateTask, item *TaskItem) {
    
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
						return
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
					return
				}
			} else {
				updateTaskStatus(item, "下载镜像", 0)
				if err := pullImageWithProgress(task.Device.IP, task.ImageName, item); err != nil {
					if err.Error() != "用户取消" {
						updateTaskStatus(item, "镜像下载失败", 1)
					} else {
						updateTaskStatus(item, "canceled", 1)
					}
					return
				}
			}
        }

        // 创建容器
        updateTaskStatus(item, "创建容器", 0.5)
        idx := findAvailableIdx(task.Device.IP)
        if idx == -1 {
            updateTaskStatus(item, "无可用坑位", 1)
            return
        }
		
		var req []byte
		switch task.Device.Name {
			case "q1_10", "c1_10", "q1", "C1", "":
				req = buildCreateRequest_rk3588(task.ImageName, idx, task.Resolution, task.Network, task.Ipaddr)
			case "p1":
				req = buildCreateRequest_cix(task.ImageName, idx, task.Resolution)
			default:
				updateTaskStatus(item, "不支持的型号", 1)
				return
		}

        if err := createAndStartContainer(idx, task.Device.IP, task.Device.ID, task.Name, req, item); err != 0 {
            updateTaskStatus(item, "创建失败: ", 1)
        } else {
            updateTaskStatus(item, "创建成功", 1)
        }
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

		switch item.Type {
        case TaskCreateContainer:
            go handleCreateTask(task,item)
        case TaskUploadFile:
            go handleUploadTask(task,item)
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
    if (item.Type == TaskUploadFile) {
		text := item.DeviceIP
		text = text[(strings.Index(text,".")+1):]
		text = text[(strings.Index(text,".")+1):]
		text = fmt.Sprintf("T%d%s %s-%s", item.Idx, status, text, filepath.Base(item.FilePath))
		if len(text) > 26 {
			text = text[0:25]
		}
		item.InfoLabel.SetText(text)
    } else {
		text := fmt.Sprintf("%s - %s", item.DeviceIP, status)
		fmt.Println("text长度:", len(text))
		if len(text) > 26 {
			text = text[0:25]
		}
		fmt.Println("text长度:", len(text))
		item.InfoLabel.SetText(text)
    }
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
			AutoRemove    bool              `json:"AutoRemove"`
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
			AutoRemove    bool              `json:"AutoRemove"`
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

// -------------------- 分组 ----------------------------
// 添加分组展开状态结构
type GroupViewState struct {
    GroupID    string
    IsExpanded bool
}

type DeviceGroup struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    Devices []string `json:"devices"` // 存储设备IP
}

type GroupList []struct{
	IsGroup bool
	Group   DeviceGroup
	Device  DeviceInfo
}

var (
    deviceGroups    []DeviceGroup
    currentGroupID  string = "default"
    groupedList     *widget.List
    groupedListData GroupList
)

var (
    groupStates   = make(map[string]bool) // 分组展开状态
    groupOrder    []string                 // 分组显示顺序
)

type clickableItem struct {
    widget.BaseWidget
    icon    *widget.Icon
    label   *widget.Label
    addBtn  *widget.Button
    onClick func()
}

func newClickableItem(icon fyne.Resource, text string, onClick func()) *clickableItem {
    c := &clickableItem{
        icon:    widget.NewIcon(icon),
        label:   widget.NewLabel(text),
        addBtn:   widget.NewButton("+",nil),
        onClick: onClick,
    }
    c.addBtn.Hide()
    c.ExtendBaseWidget(c)
    return c
}

func (c *clickableItem) CreateRenderer() fyne.WidgetRenderer {
    return widget.NewSimpleRenderer(
        container.NewHBox(c.icon, c.label, layout.NewSpacer(), c.addBtn),
    )
}

func (c *clickableItem) Tapped(*fyne.PointEvent) {
    if c.onClick != nil {
        c.onClick()
    }
}

// 修改设备列表创建方式
func createCollapsibleDeviceList() fyne.CanvasObject {
    groupedListData = buildNestedListData("")
    groupedList = widget.NewList(
		func() int { return len(groupedListData) },
		func() fyne.CanvasObject {
			return newClickableItem(nil, "", nil) // 初始化空项
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := groupedListData[id]
			ci := obj.(*clickableItem)

			if item.IsGroup {
				ci.addBtn.Show()
				ci.addBtn.OnTapped = func() {
					fmt.Println("点击了添加分组",item.Group.ID)
					addDevice2Groups(item.Group.ID)
				}
				// 分组项处理
				if groupStates[item.Group.ID] {
					ci.icon.SetResource(theme.MenuDropDownIcon())
				} else {
					ci.icon.SetResource(theme.MenuExpandIcon())
				}
				ci.label.SetText(fmt.Sprintf("%s (%d)", item.Group.Name, len(item.Group.Devices)))
				ci.onClick = func() {
					groupStates[item.Group.ID] = !groupStates[item.Group.ID]
					groupedListData = buildNestedListData("")
					groupedList.Refresh()
					
				}
			} else {
				// 设备项处理
				if selectedDevice == item.Device.IP {
					ci.icon.SetResource(theme.ConfirmIcon())
				} else {
					ci.icon.SetResource(theme.ComputerIcon())
				}
				ci.label.SetText(fmt.Sprintf("  %s - %s", item.Device.IP, item.Device.Name))
				ci.onClick = func() {
					// 设备点击处理
					selectedDevice = item.Device.IP
					groupedList.Refresh()
					refreshContainers()
					ci.icon.SetResource(theme.ConfirmIcon())
				}
			}
		},
	)

    return groupedList
}

// 构建嵌套列表数据结构
func buildNestedListData(filter string) GroupList {
    var data GroupList

    for _, group := range deviceGroups {
        // 添加分组标题
        data = append(data, struct{
            IsGroup bool
            Group   DeviceGroup
            Device  DeviceInfo
        }{true, group, DeviceInfo{}})

		// 查询分组
		//if filter != ""  {
        //    groupStates[group.ID] = true
        //}
        for _, id := range group.Devices {
            if d, found := getDeviceByID(id); found {
                if strings.Contains(d.IP, filter) || 
				   strings.Contains(d.Name, filter) || 
				   strings.Contains(d.Type, filter) || strings.Contains(group.Name, filter) {
					// 展示分组设备（如果展开）
					if groupStates[group.ID] || filter != "" {
						groupStates[group.ID] = true
						data = append(data, struct{
							IsGroup bool
							Group   DeviceGroup
							Device  DeviceInfo
						}{false, DeviceGroup{}, d})
					}
				} else if filter != "" {
					groupStates[group.ID] = false
				}
            }
        }
    }
    return data
}

func getDeviceByID(id string) (DeviceInfo, bool) {
	for _, d := range savedDevices {
        if d.ID == id {
            return d, true
        }
    }
    return DeviceInfo{}, false
}

// 在初始化时设置默认展开状态
func initGroupStates() {
    for _, g := range deviceGroups {
        groupStates[g.ID] = false // 默认折叠
    }
    groupStates["default"] = true // 默认分组展开
}

// 修改后的创建分组对话框
func createGroupManagement() {
    nameEntry := widget.NewEntry()
    
    dialog.ShowForm("新建分组", "创建", "取消",
        []*widget.FormItem{
            {Text: "分组名称", Widget: nameEntry},
        },
        func(confirm bool) {
            if !confirm {
                return
            }
            
            // 检查重名
            for _, group := range deviceGroups {
                if group.Name == nameEntry.Text {
                    dialog.ShowInformation("错误", "组名已存在", mainWindow)
                    return
                }
            }
            
            // 创建新组
            deviceGroups = append(deviceGroups, DeviceGroup{
                ID:      fmt.Sprintf("group-%d", time.Now().UnixNano()),
                Name:    nameEntry.Text,
                Devices: []string{},
            })
            saveData()
            
            // 刷新界面
            groupedListData = buildNestedListData("")
            groupedList.Refresh()
        },
        mainWindow)
}

// 在初始化时添加默认分组
func initGroups() {
    // 检查是否已经存在默认分组
    defaultExists := false
    for _, g := range deviceGroups {
        if g.ID == "default" {
            defaultExists = true
            break
        }
    }

    // 创建默认分组（如果不存在）
    if !defaultExists {
        deviceGroups = append(deviceGroups, DeviceGroup{
            ID:     "default",
            Name:   "默认分组",
            Devices: []string{},
        })
    }

    // 获取所有未被分组的设备
    groupedDevices := make(map[string]bool)
    for _, group := range deviceGroups {
        if group.ID != "default" {
            for _, devID := range group.Devices {
                groupedDevices[devID] = true
            }
        }
    }

    // 找到默认分组的索引
    defaultIndex := -1
    for i, g := range deviceGroups {
        if g.ID == "default" {
            defaultIndex = i
            break
        }
    }

    // 添加未分组的设备到默认分组
    if defaultIndex != -1 {
        var ungrouped []string
        for _, d := range savedDevices {
            if !groupedDevices[d.ID] {
                ungrouped = append(ungrouped, d.ID)
            }
        }
        deviceGroups[defaultIndex].Devices = ungrouped
    }
    saveData()
}

func addDevice2Groups(existingGroupID string) {
    // 获取目标分组
    var targetGroup *DeviceGroup
    //groupIndex := -1
    for i, g := range deviceGroups {
        if g.ID == existingGroupID {
            targetGroup = &deviceGroups[i]
            //groupIndex = i
            break
        }
    }

    // 创建设备选择状态映射
    selectedDevices := make(map[string]bool)
    for _, devID := range targetGroup.Devices {
        selectedDevices[devID] = true
    }

    list := widget.NewList(
        func() int { return len(savedDevices) },
        func() fyne.CanvasObject {
            return container.NewHBox(
                widget.NewCheck("", nil),
                widget.NewLabel(""),
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            device := savedDevices[i]
            cont := o.(*fyne.Container)
            check := cont.Objects[0].(*widget.Check)
            label := cont.Objects[1].(*widget.Label)

            // 初始选中状态
            check.Checked = selectedDevices[device.ID]
            check.Refresh()

            check.OnChanged = func(checked bool) {
                selectedDevices[device.ID] = checked
            }

            label.SetText(fmt.Sprintf("%s (%s)", device.IP, device.Name))
        },
    )

    dialog.ShowCustomConfirm(
        "管理分组设备", 
        "保存", 
        "取消",
        container.NewVScroll(list),
        func(confirm bool) {
            if confirm && targetGroup != nil {
                // 生成新的设备列表
                var newDevices []string
                for _, d := range savedDevices {
                    if selectedDevices[d.ID] {
                        newDevices = append(newDevices, d.ID)
                    }
                }
                
                // 更新分组设备（会自动处理默认分组）
                addDeviceTool(targetGroup.Name, newDevices)
                
                // 刷新界面
                groupedListData = buildNestedListData("")
                groupedList.Refresh()
            }
        },
        mainWindow,
    )
}

// 添加设备到组（带组存在性检查）
func addDeviceTool(groupName string, ids []string) {
    // 找到目标分组
    targetGroupIndex := -1
    for i, group := range deviceGroups {
        if group.Name == groupName {
            targetGroupIndex = i
            break
        }
    }

    // 如果分组不存在则创建
    if targetGroupIndex == -1 {
        deviceGroups = append(deviceGroups, DeviceGroup{
            ID:      fmt.Sprintf("group-%d", time.Now().UnixNano()),
            Name:    groupName,
            Devices: ids,
        })
        targetGroupIndex = len(deviceGroups)-1
    } else {
        // 直接替换设备列表（不再合并）
        deviceGroups[targetGroupIndex].Devices = ids
    }

    // 从默认分组移除这些设备
    defaultGroupIndex := -1
    for i, g := range deviceGroups {
        if g.ID == "default" {
            defaultGroupIndex = i
            break
        }
    }
    
    if defaultGroupIndex != -1 {
        remaining := []string{}
        for _, devID := range deviceGroups[defaultGroupIndex].Devices {
            if !contains(ids, devID) {
                remaining = append(remaining, devID)
            }
        }
        deviceGroups[defaultGroupIndex].Devices = remaining
    }

    saveData()
}

func getAllDeviceIDs() []string {
    ids := make([]string, len(savedDevices))
    for i, d := range savedDevices {
        ids[i] = d.ID
    }
    return ids
}

// -------------------- 主函数和初始化 --------------------
func main() {
	mainApp = app.New()
	mainWindow = mainApp.NewWindow("边缘计算管理平台 v0.5")
	mainWindow.Resize(fyne.NewSize(1280, 720))

	// 加载设备列表
	if err := loadDevices(); err != nil {
        fmt.Println("加载设备失败:", err)
    }
    // 启动状态检查
    startStatusChecker()
	loadData()
	initGroups()
	initGroupStates()

	// 创建界面组件
	searchEntry := widget.NewEntry()
    searchEntry.SetPlaceHolder("搜索设备...")
    searchEntry.OnChanged = func(s string) {
        groupedListData = buildNestedListData(s)
        groupedList.Refresh()
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
	
	// 添加设备按钮
    addBtn := widget.NewButtonWithIcon("添加设备", theme.ContentAddIcon(), createAddDeviceDialog)

    // 修改布局添加间隔选择器
    mainContent := container.NewHSplit(
        container.NewBorder(
            container.NewVBox(widget.NewLabel("设备列表"),
				container.NewAdaptiveGrid(2,
					searchEntry,
					container.NewAdaptiveGrid(2,
						widget.NewButtonWithIcon("组", theme.ContentAddIcon(), func() {
							createGroupManagement()
						}),
						widget.NewButton("刷新", func() {
							searchEntry.SetText("")
							refreshDevices()
						}),
					),
				),
			),
            addBtn,
            nil, nil,
            container.NewVScroll(createCollapsibleDeviceList()),
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

	// 初始加载
	go refreshDevices()
	go processTaskQueue()
	go fetchImageList("q1")
	
	mainWindow.SetContent(mainTabs)
	mainWindow.ShowAndRun()
}
