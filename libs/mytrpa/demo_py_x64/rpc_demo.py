from common.mytRpc import MytRpc
from common.mytSelector import mytSelector
from common.rpcNode import rpcNode
import cv2
#import np
import time
import sys
if __name__ == '__main__':
    for arg in sys.argv[1:]:
        print(f"参数: {arg}")

    # 申明对象
    mytapi = MytRpc()
    sdk_ver = mytapi.get_sdk_version()
    print(f"SDK 版本号:  {sdk_ver}")
    # 连接设备 在api 列表里查看辅助控制API ip 和端口
	#***********************************************
	#注意!!! 注意!!! 注意!!!
	#这里的端口不是adb 的端口  请在客户端上查看API列表里面获取  辅助控制API 的端口
	#***********************************************
    #print(f"{host_ip}:{host_port}")
    # while 1:
    #     print("child \n")
    #     time.sleep(2)

    if mytapi.init("192.168.181.27", 11010, 10) == True:
        print("连接设备设备成功!")

        # 以下展示了所有功能演示代码 可以打开注释 逐一测试

        #测试连接性
        if mytapi.check_connect_state() == True:
            print("当前连接状态正常")
        else:
             print("当前连接断开")
        

        if mytapi.setRpaWorkMode(0):
            print("设置工作模式为 关闭无障碍")
        
        #mytapi.longClick(1, 100, 100, 1.0)
        #mytapi.screentshot(1, 90, "d:/1.png")
        # 1   截图操作
        # byt_arr = mytapi.takeCaptrueCompress(0, 100)
        # if len(byt_arr)>0:
        #     # 显示图像
        #     img_np = np.frombuffer(byt_arr, dtype=np.uint8)
        #     img = cv2.imdecode(img_np, cv2.IMREAD_COLOR)
        #     cv2.imshow("Image", img)
        #     cv2.waitKey(0)
        # else:
        #     print("获取截图失败")
        # 1.1 截图并保存到本地
        # for i in range(0, 30):
        #     mytapi.screentshotEx(i, i, 500 + i,500 + i, 1, 90, f"d:/{i}.png")
        #     time.sleep(0.5)



        # ret = mytapi.exec_cmd("ls /data")
        # print(ret)
        #2 导出节点信息
        # str = mytapi.dumpNodeXml(True)
        # if str == False:
        #     print("导出失败 请重试!")
        # else:
        #     #print(str)
        #     #写入文件
        #     with open("d:/node.xml", "w", encoding='utf-8') as f:
        #         f.write(str)

        #3 打开应用
        # if mytapi.openApp("com.blue.filemanager") == False:
        #     print("运行 com.blue.filemanager 失败!")
        # else:
        #     print("运行 com.blue.filemanager 成功!")

        #4 关闭应用
        # if mytapi.stopApp("com.blue.filemanager") == False:
        #     print("停止 com.blue.filemanager 失败!")
        # else:
        #     print("停止 com.blue.filemanager 成功!")
        

        #5 输入文件
        # if mytapi.sendText("中文测试!") == False:
        #     print("发送文字 失败!")
        # else:
        #     print("发送文字 成功!")  
        
        #mytapi.ClearText(10)
        #print(mytapi.getNodeByClass("android.widget.FrameLayout"))
        #6 执行adb命令行
        # str = mytapi.exec_cmd(" dumpsys activity  ")
        # if str != False:
        #     print(str)
        
        
        #7 移动操作
        #从(500,700) 滑动到(500,1200)
        #起点500 1260  终点500 200
        # y= 1260
        # finger_id =0
        # mytapi.touchDown(finger_id, 500, y)
        # for i in range(500):
        #     y=y-2
        #     mytapi.touchMove(finger_id, 500, y)
        #     print(f"500  - {y}")
        #     time.sleep(0.08)                         #控制速度
        # mytapi.touchUp(finger_id, 500, y)
        #mytapi.swipe(1, 100,100, 500,540, 4000)


        #8 按键操作
        #     KEYCODE_CALL 拨号键5
        # KEYCODE_ENDCALL 挂机键6
        # KEYCODE_HOME 按键Home3
        # KEYCODE_MENU 菜单键82
        # KEYCODE_BACK 返回键4
        # KEYCODE_SEARCH 搜索键84
        # KEYCODE_CAMERA 拍照键27
        # KEYCODE_FOCUS 拍照对焦键80
        # KEYCODE_POWER 电源键26
        # KEYCODE_NOTIFICATION 通知键83
        # KEYCODE_MUTE 话筒静音键91
        # KEYCODE_VOLUME_MUTE 扬声器静音键164
        # KEYCODE_VOLUME_UP 音量增加键24
        # KEYCODE_VOLUME_DOWN 音量减小键25


        # KEYCODE_ENTER 回车键66
        # KEYCODE_ESCAPE ESC键111
        # KEYCODE_DPAD_CENTER 导航键 确定键23
        # KEYCODE_DPAD_UP 导航键 向上19
        # KEYCODE_DPAD_DOWN 导航键 向下20
        # KEYCODE_DPAD_LEFT 导航键 向左21
        # KEYCODE_DPAD_RIGHT 导航键 向右22
        # KEYCODE_MOVE_HOME 光标移动到开始键122
        # KEYCODE_MOVE_END 光标移动到末尾键123
        # KEYCODE_PAGE_UP 向上翻页键92
        # KEYCODE_PAGE_DOWN 向下翻页键93
        # KEYCODE_DEL 退格键67
        # KEYCODE_FORWARD_DEL 删除键112
        # KEYCODE_INSERT 插入键124
        # KEYCODE_TAB Tab键61
        # KEYCODE_NUM_LOCK 小键盘锁143
        # KEYCODE_CAPS_LOCK 大写锁定键115
        # KEYCODE_BREAK Break/Pause键121
        # KEYCODE_SCROLL_LOCK 滚动锁定键116
        # KEYCODE_ZOOM_IN 放大键168
        # KEYCODE_ZOOM_OUT 缩小键169

        #音量 +
        # if mytapi.keyPress(24) == False:
        #     print('失败!')
        # else:
        #     print('成功!')

        #节点处理
        # selector = mytapi.create_selector() 
        # with selector:
        #     selector.addQuery_TextContainWith('导入')
        #     #selector.addQuery_Enable(False)
        #     node = selector.execQueryOne(200)
        #     if node is not None:
        #         node.Click_events()
        #         print(node.getNodeJson())
        
        

    else:
        print("连接设备设备 可能是ip 或者端口错误 或者设备没有开机等原因!")

