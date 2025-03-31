import ctypes
import sys
import time
import os
from common.logger import logger
from common.ToolsKit import ToolsKit
from common.mytSelector import mytSelector
import json


CB_FUNC = ctypes.CFUNCTYPE(ctypes.c_void_p, ctypes.c_int, ctypes.POINTER(ctypes.c_void_p), ctypes.c_int)
AUDIO_CB_FUNC = ctypes.CFUNCTYPE(ctypes.c_void_p, ctypes.POINTER(ctypes.c_void_p), ctypes.c_int)


@CB_FUNC
def video_cb(rot, data, len):
    # 在这里处理接收到的数据
    buf = ctypes.cast(data, ctypes.POINTER(ctypes.c_ubyte * len)).contents
    bin_buf = bytearray(buf)
    # 此次为解析出来的h264流数据  可以做相应的操作处理 这里只是给出保存到文件的示例
    with open("video.raw", 'ab') as f:
        f.write(bin_buf)
    # print("video",rot, data, len)
    # res = ctypes.string_at(data, len)
    # g_data += res
    # if time.time() - start < 10:
    #     with open("video.raw", 'ab+') as f:
    #         f.write(res)
# cb = CB_FUNC(cb1)


#播放acc 文件  就添加头  如果直接解码 就不需要添加adts 头
def add_adts_header(aac_data):
    # ADTS 头部格式
    adts = [0] * 7
    # ADTS 头部详细参数
    profile = 1  # AAC LC (Low Complexity) profile is 1
    freq_idx = 4            #44100
    chan_cfg = 2            #channels =2 
    # 计算帧长度
    frame_length = len(aac_data) 
    # 构造 ADTS 头部
    adts[0] = 0xFF  # 同步字
    adts[1] = 0xF1  # 同步字，MPEG-2 Layer (0 for MPEG-4)，保护标志
    adts[2] = (profile << 6) + (freq_idx << 2) + (chan_cfg >> 2)
    adts[3] = ((chan_cfg & 3) << 6) + ((frame_length + 7) >> 11)
    adts[4] = ((frame_length + 7) & 0x7FF) >> 3
    adts[5] = (((frame_length + 7) & 7) << 5) + 0x1F
    adts[6] = 0xFC  # Number of raw data blocks in frame
    # 合并 ADTS 头部和 AAC 数据
    adts_aac_data = bytearray(adts) + aac_data
    return adts_aac_data

@AUDIO_CB_FUNC
def audio_cb(data, len):
    
    if len == 2:
        #该2个字节为myt 添加的标记 不用处理 
        #print(f"audio_cb :len={len}")
        pass
    else:
        buf = ctypes.cast(data, ctypes.POINTER(ctypes.c_ubyte * len)).contents
        bin_buf = bytearray(buf)

        #播放acc 文件  就添加头  如果直接解码 就不需要添加adts 头
        adts_aac_data = add_adts_header(bin_buf)
        # 此次为解析出来的aac 原始音频流数据  可以做相应的操作处理 这里只是给出保存到文件的示例
        with open("audio.aac", 'ab') as f:
            f.write(adts_aac_data)

# myt rpc  lib
#   add node oper 2024.1.31
class MytRpc(object):
    # _lib_PATH = "/home/zwj/tiktok/lib/libmytrpc.so"
    # _handle = 0
    def __init__(self) -> None:

        tools = ToolsKit()
        root_path = tools.GetRootPath()
        if sys.platform == "linux":
            #self._lib_PATH = os.path.dirname(os.path.abspath(__file__)) + "/../lib/libmytrpc.so"
            self._lib_PATH = root_path + "/lib/libmytrpc.so"
        elif sys.platform == "darwin":
            self._lib_PATH = root_path + "/lib/libmytrpc.dylib"
        else:
            self._lib_PATH = root_path + "/lib/libmytrpc.dll"
             #self._lib_PATH = os.path.dirname(os.path.abspath(__file__)) + "/../lib/libmytrpc.dll"
        self._handle = 0
        # if os.path.exists(self._lib_PATH) == False:
        #     self._lib_PATH = os.path.dirname(os.path.abspath(__file__)) + "/libmytrpc.so"

    def __del__(self) :
        if self._handle>0 :
            self._rpc.closeDevice(self._handle)
    
    #获取SDK 版本
    def get_sdk_version(self):
        ret = ''
        if os.path.exists(self._lib_PATH) == True:
            if sys.platform == "linux":
                dll = ctypes.CDLL(self._lib_PATH)
            elif sys.platform == "darwin":
                dll = ctypes.CDLL(self._lib_PATH)
            else:
                dll = ctypes.WinDLL(self._lib_PATH)

            ret = dll.getVersion()
        return ret

    # 初始化
    def init(self, ip, port, timeout):
        ret = False
        if os.path.exists(self._lib_PATH) == True:
            if sys.platform == "linux":
                self._rpc = ctypes.CDLL(self._lib_PATH)
            elif sys.platform == "darwin":
                self._rpc = ctypes.CDLL(self._lib_PATH)
            else:
                self._rpc = ctypes.WinDLL(self._lib_PATH)
            
            b_time = int(time.time())
            while True:
                self._handle = self._rpc.openDevice(bytes(ip, "utf-8"), port, 10)
                if self._handle > 0:
                    ret = True
                    logger.debug("rpc " + ip + " ok!")
                    break
                else:
                    now = int(time.time())
                    if now-b_time>timeout :
                        logger.debug("rpc " + ip + " timeout " + str(timeout))
                        break
                    else:
                        time.sleep(10)
        else:
            logger.debug("File not Found: " + self._lib_PATH)
        return ret

    #检查远程连接是否处于连接状态
    def check_connect_state(self):
        ret = False
        if self._handle>0:
            self._rpc.checkLive.argtypes = [ctypes.c_long]
            self._rpc.checkLive.restype = ctypes.c_int
            exec_ret = self._rpc.checkLive(self._handle)
            if exec_ret == 0:
                ret = False
            else:
                ret = True
        return ret
            #LIBMYTRPC_API int MYAPI checkLive(long handle);

    # 执行命令
    #返回状态值 和 内容
    def exec_cmd(self, cmd):
        ret = False
        out_put = ''
        if self._handle > 0:
            # cmd = " pm install /data/local/TikTok_26.5.3_apkcombo.com.apk"
            # cmd = "ls"
            self._rpc.execCmd.restype = ctypes.c_char_p
            ptr = self._rpc.execCmd(self._handle, ctypes.c_int(1), ctypes.c_char_p(cmd.encode('utf-8'))) 
            if ptr is not None:
                #if isinstance(ptr, str):
                out_put = ptr.decode('utf-8')
                logger.debug("exec " + cmd + "  :" + out_put)
                # else:
                ret = True
            else:
                ret = True
        return out_put, ret

    # 导出节点信息
    # bDumpAll 导出所有节点  0   1

    def dumpNodeXml(self, bDumpAll):
        """
        导出节点XML信息。

        参数:
        - bDumpAll (bool): 是否导出所有信息，True表示导出全部，False表示仅导出部分信息。

        返回:
        - ret: 成功时返回XML字符串，失败时返回False。
        """
        ret = False
        # 确保_handle有效，以防止在无效的状态下调用RPC方法
        if self._handle > 0:
            # 设置RPC方法dumpNodeXml的参数类型和返回类型
            self._rpc.dumpNodeXml.argtypes = [ctypes.c_long, ctypes.c_int]
            self._rpc.dumpNodeXml.restype = ctypes.c_void_p
            # 调用RPC方法dumpNodeXml获取节点XML信息的指针
            ptr = self._rpc.dumpNodeXml(self._handle, bDumpAll)
            if ptr:
                # 将指针转换为字符串
                p2 = ctypes.cast(ptr, ctypes.c_char_p)
                ret = p2.value.decode("utf-8")
                # 释放指针内存，以防止内存泄漏
                self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
                self._rpc.freeRpcPtr(ptr)
            else:
                # 当返回的指针为空时，记录调试信息
                logger.debug('dumpNodeXml is NULL ptr!')
        return ret

    def dumpNodeXmlEx(self, workMode, timeout):
            """
            导出节点XML信息。

            参数:
               workMode True  表示开启无障碍  
                         False   表示关闭无障碍 
                timeout   超时  单位豪秒(1秒=1000毫秒)  -1 为永不超时

            返回:
            - ret: 成功时返回XML字符串，失败时返回False。
            原型
            LIBMYTRPC_API char* MYAPI dumpNodeXmlEx(long handle, int useNewMode, int timeout);
            """
            ret = False
            # 确保_handle有效，以防止在无效的状态下调用RPC方法
            if self._handle > 0:
                # 设置RPC方法dumpNodeXml的参数类型和返回类型
                self._rpc.dumpNodeXmlEx.argtypes = [ctypes.c_long, ctypes.c_int, ctypes.c_int]
                self._rpc.dumpNodeXmlEx.restype = ctypes.c_void_p
                # 调用RPC方法dumpNodeXml获取节点XML信息的指针
                if workMode == True:
                    iMode = 1
                else:
                    iMode = 0
                ptr = self._rpc.dumpNodeXmlEx(self._handle, iMode, timeout)
                if ptr:
                    # 将指针转换为字符串
                    p2 = ctypes.cast(ptr, ctypes.c_char_p)
                    ret = p2.value.decode("utf-8")
                    # 释放指针内存，以防止内存泄漏
                    self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
                    self._rpc.freeRpcPtr(ptr)
                else:
                    # 当返回的指针为空时，记录调试信息
                    logger.debug('dumpNodeXmlEx is NULL ptr!')
            return ret

    # 截图导出为bytes 数组
    # type  0 png  1 jpg
    # quality  图片质量  0-100
    # 返回字节数组
    def takeCaptrueCompress(self, type, quality):
        ret = False
        if self._handle > 0:
            dataLen = ctypes.c_int(0)
            self._rpc.takeCaptrueCompress.argtypes = [ctypes.c_long, ctypes.c_int, ctypes.c_int,  ctypes.POINTER(ctypes.c_int)]
            self._rpc.takeCaptrueCompress.restype = ctypes.c_void_p
            ptr = self._rpc.takeCaptrueCompress(self._handle, type, quality, ctypes.byref(dataLen))
            if ptr:
                try:
                    # 将ptr转换为bytes对象
                    buf = ctypes.cast(ptr, ctypes.POINTER(ctypes.c_ubyte * dataLen.value)).contents
                    # 使用OpenCV将bytes对象转换为图像
                    ret = bytearray(buf)
                    # img_np = np.frombuffer(img_array, dtype=np.uint8)
                    # img = cv2.imdecode(img_np, cv2.IMREAD_COLOR)
                    # # 显示图像
                    # cv2.imshow("Image", img)
                    # cv2.waitKey(0)
                finally:
                    # 释放指针内存
                    self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
                    self._rpc.freeRpcPtr(ptr)
        return ret
    
    #指定区域截图
    def takeCaptrueCompressEx(self, left, top, right, bottom, type, quality):
        ret = False
        if self._handle > 0:
            dataLen = ctypes.c_int(0)
            self._rpc.takeCaptrueCompressEx.argtypes = [ctypes.c_long, ctypes.c_int, ctypes.c_int,ctypes.c_int, ctypes.c_int,ctypes.c_int, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
            self._rpc.takeCaptrueCompressEx.restype = ctypes.c_void_p
            ptr = self._rpc.takeCaptrueCompressEx(self._handle, left, top,right, bottom, type, quality, ctypes.byref(dataLen))
            if ptr:
                try:
                    # 将ptr转换为bytes对象
                    buf = ctypes.cast(ptr, ctypes.POINTER(ctypes.c_ubyte * dataLen.value)).contents
                    # 使用OpenCV将bytes对象转换为图像
                    ret = bytearray(buf)
                    # img_np = np.frombuffer(img_array, dtype=np.uint8)
                    # img = cv2.imdecode(img_np, cv2.IMREAD_COLOR)
                    # # 显示图像
                    # cv2.imshow("Image", img)
                    # cv2.waitKey(0)
                finally:
                    # 释放指针内存
                    self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
                    self._rpc.freeRpcPtr(ptr)
            else:
                logger.debug(f"takeCaptrueCompressEx error {ptr}")
        return ret
    
    
    #截图到文件
    def screentshotEx(self,left, top, right, bottom, type, quality, file_path):
        ret = False
        byte_data = self.takeCaptrueCompressEx(left, top, right, bottom,type, quality) 
        if byte_data != False:
            with open(file_path, 'wb') as file:
                file.write(byte_data)
            ret = True
        else:
            logger.debug("screentshotEx error")
        return ret

    #截图到文件
    def screentshot(self, type, quality, file_path):
        ret = False
        byte_data = self.takeCaptrueCompress(type, quality) 
        if byte_data != False:
            with open(file_path, 'wb') as file:
                file.write(byte_data)
            ret = True
        return ret

    # 文字输入
    def sendText(self, text):
        ret = False
        if self._handle > 0:
            exec_ret =  self._rpc.sendText(self._handle, ctypes.c_char_p(text.encode('utf-8')))
            if exec_ret == 1:
                ret = True
        return ret

    #清除输入的文字
    def ClearText(self, count):
        for i in range(0, count):
            self.keyPress(67)
        


    # 开启指定的应用
    def openApp(self, pkg):
        ret = False
        if self._handle > 0:
            exec_ret = self._rpc.openApp(self._handle, ctypes.c_char_p(pkg.encode('utf-8')))
            if exec_ret == 0:
                ret = True
        return ret

    #停止指定的应用
    def stopApp(self, pkg):
        ret = False
        if self._handle > 0:
            exec_ret = self._rpc.stopApp(self._handle, ctypes.c_char_p(pkg.encode('utf-8')))
            if exec_ret == 0:
                ret = True
        return ret

    #获取当前屏幕的方向
    #  4个方向（0,1,2,3）
    def getDisplayRotate(self):
        ret = False
        if self._handle > 0:
            if self._rpc.getDisplayRotate(self._handle) == 1:
                ret = True
        return ret
    
    #按下操作
    def touchDown(self, finger_id, x, y):
        ret = False
        if self._handle > 0:
            if self._rpc.touchDown(self._handle, finger_id, x, y) == 1:
                ret = True
        return ret
    
    #弹起操作
    def touchUp(self, finger_id, x, y):
        ret = False
        if self._handle > 0:
            if self._rpc.touchUp(self._handle, finger_id, x, y) == 1:
                ret = True
        return ret
    
    #滑动操作
    def touchMove(self, finger_id, x, y):
        ret = False
        if self._handle > 0:
            if self._rpc.touchMove(self._handle, finger_id, x, y) == 1:
                ret = True
        return ret
    
    #单击操作
    def touchClick(self, finger_id, x, y):
        ret = False
        if self._handle > 0:
            if self._rpc.touchClick(self._handle, finger_id, x, y) == 1:
                ret = True
        return ret

    #长按操作
    #t 为长按的时长  单位: 秒(float)
    def longClick(self, finger_id, x, y, t):
        ret = False
        if self._handle>0:
            if self._rpc.touchDown(self._handle, finger_id, x, y) > 0:
                time.sleep(t)
                exec_ret = self._rpc.touchUp(self._handle, finger_id, x, y)
                if exec_ret ==1 :
                    ret = True
        return ret

    #按键操作
    # 键值 参考: https://blog.csdn.net/yaoyaozaiye/article/details/122826340
    def keyPress(self, code):
        ret = False
        if self._handle > 0:
            if self._rpc.keyPress(self._handle, code) == 1:
                ret = True
        return ret
    
    #Back按键
    def pressBack(self):
        return self.keyPress(4)
    
    #Enter 按键
    def pressEnter(self):
        return self.keyPress(66)
    
    #Home 按键
    def pressHome(self):
        return self.keyPress(3)
    
    #Menu 按键
    def pressRecent(self):
        return self.keyPress(82)

    #滑动操作
    # x0 y0 起始坐标
    # x1 y1 终点坐标
    # elapse 时长  (单位:毫秒)   
    def swipe(self, id, x0, y0, x1, y1, elapse):
        ret = False
        if self._handle>0:
            ret = self._rpc.swipe(self._handle,id,  x0, y0, x1, y1, elapse, False)
        return ret
    
    #创建selector筛选器对象
    def create_selector(self):
        ret = None
        if self._handle>0:
            ret = mytSelector(self._handle, self._rpc)
        return ret
    
    #释放 selector 对象
    def release_selector(self, sel):
        del sel
    
    #按照Node的属性执行点击
    def clickText(self, text):
        ret = False
        selector = self.create_selector() 
        selector.addQuery_TextEqual(text)
        node = selector.execQueryOne(200)
        if node is not None:
            ret = node.Click_events()
        self.release_selector(selector)
        return ret

    def clickTextMatchStart(self, text):
        ret = False
        selector = self.create_selector()
        selector.addQuery_TextStartWith(text)
        node = selector.execQueryOne(200)
        if node is not None:
            ret = node.Click_events()
        self.release_selector(selector)
        return ret
        
    def clickClass(self, clzName):
        ret = False
        selector = self.create_selector() 
        selector.addQuery_ClzEqual(clzName)
        node = selector.execQueryOne(200)
        if node is not None:
            ret = node.Click_events()
        self.release_selector(selector)
        return ret
    
    def clickId(self, id):
        ret = False
        selector = self.create_selector() 
        selector.addQuery_IdEqual(id)
        node = selector.execQueryOne(200)
        if node is not None:
            ret = node.Click_events()
        self.release_selector(selector)
        return ret
    
    def clickDesc(self, des):
        ret = False
        selector = self.create_selector() 
        selector.addQuery_DescEqual(des)
        node = selector.execQueryOne(200)
        if node is not None:
            ret = node.Click_events()
        self.release_selector(selector)
        return ret

    #依据Text 获取Node节点
    def getNodeByText(self, text):
        ret  = None
        selector = self.create_selector() 
        selector.addQuery_TextEqual(text)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret

    def getNodeByTextMatchEnd(self, text):
        ret  = None
        selector = self.create_selector()
        selector.addQuery_TextEndWith(text)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret

    def getNodeByTextMatchStart(self, text):
        ret  = None
        selector = self.create_selector()
        selector.addQuery_TextStartWith(text)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret

    #根据pkg 获取Node节点
    def getNodeByPkg(self,pkg):
        ret = None
        selector = self.create_selector()
        selector.addQuery_PackageEqual(pkg)
        node_arr = selector.execQuery(999, 200)
        if len(node_arr) > 0:
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret

    def getNodeByClass(self, clzName):
        ret  = None
        selector = self.create_selector() 
        selector.addQuery_ClzEqual(clzName)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret

    def getNodeById(self, id):
        ret  = None
        selector = self.create_selector() 
        selector.addQuery_IdEqual(id)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret
    
    def getNodeByDesc(self, desc):
        ret  = None
        selector = self.create_selector() 
        selector.addQuery_DescEqual(desc)
        node_arr = selector.execQuery(999,200)
        if len(node_arr)>0 :
            arr = []
            for n in node_arr:
                json_str = n.getNodeJson()
                json_obj = json.loads(json_str)
                arr.append(json_obj)
            ret = json.dumps(arr)
        self.release_selector(selector)
        return ret
    
    #设置rpa 的工作模式     1   表示开启无障碍  （默认的工作模式）   
    #                      0   表示关闭无障碍 
    # 设置RPA工作模式
    # 本函数用于设置RPA的工作模式，主要是为了确定是否使用无障碍模式
    # 开启无障碍模式后，可以获取更加完整的节点信息，但某些应用环境会检测是否开启了无障碍
    # 该方法需要 最新的额固件版本支持  
    # 参数:
    #   mode: 工作模式的设置值，决定是否使用无障碍模式
    # 返回值:
    #   成功设置返回True，否则返回False
    def setRpaWorkMode(self, mode):
        # 初始化返回值为False
        ret = False
        # 检查_handle是否有效
        if self._handle>0:
            # 设置useNewNodeMode函数的参数类型
            self._rpc.useNewNodeMode.argtypes = [ctypes.c_long, ctypes.c_int]
            # 设置useNewNodeMode函数的返回类型
            self._rpc.useNewNodeMode.restype = ctypes.c_int
            # 调用useNewNodeMode函数，并传入_handle和mode参数
            exec_ret = self._rpc.useNewNodeMode(self._handle, mode)
            # 根据函数执行结果设置返回值
            if exec_ret == 0:
                ret = False
            else:
                ret = True
        # 返回设置结果
        return ret

    def startVideoStream(self):
        """
        启动视频流。

        在调用此方法之前，需要确保已经成功连接到设备，并且_handle是有效的。
        该方法将根据指定的参数（分辨率和比特率）启动视频流，并注册回调函数以处理视频和音频数据。
        
        video_cb 视频回调函数
        audio_cb 音频回调函数
        Returns:
            bool: 如果视频流成功启动并运行，则返回True；否则返回False。
        """
        # 检查_handle是否有效
        if self._handle > 0:
            # 配置startVideoStream的参数类型
            self._rpc.startVideoStream.argtypes = [ctypes.c_long,
                                                ctypes.c_int,
                                                ctypes.c_int,
                                                ctypes.c_int,
                                                CB_FUNC,
                                                AUDIO_CB_FUNC
                                                ]
            # 设置startVideoStream的返回类型
            self._rpc.startVideoStream.restype = ctypes.c_int

            # 调用 startVideoStream 函数
            w = 400
            h = 720
            bitrate = 1000 * 20
            exec_ret = self._rpc.startVideoStream(self._handle, w, h, bitrate, video_cb, audio_cb)
            if exec_ret == 1:
                # 如果视频流成功启动，进入循环以持续运行
                while True:
                    time.sleep(1)
                    print('is running')
            else:
                # 如果视频流启动失败，设置返回值为False
                ret = False
        else:
            # 如果_handle无效，设置返回值为False
            ret = False
        # 返回视频流的运行状态
        return ret
        
