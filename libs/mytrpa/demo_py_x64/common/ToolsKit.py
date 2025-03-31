import os
import sys
import psutil
import platform
import requests
import json
import hashlib
import time

class ToolsKit(object):
    #获取当前程序的可执行文件路径
    def GetRootPath(self):

        #in bundle
        args = sys.argv[0]
        #判断是否 是相对路径 还是绝对路径
        if not os.path.exists(args): 
            pwd = os.getcwd()
            bundle_dir = pwd + args
        else:
            bundle_dir = os.path.dirname(args)
            if bundle_dir == '':
              bundle_dir = os.getcwd()
        return bundle_dir
    
    #判断是否多次运行
    #return   True 存在相同进程实例  False
    
    def check_multi_run(self):
        if platform.machine() == 'aarch64':         #arm板 是容器部署不进行判断
            ret = False
        else:
            ret = None
            root_path = self.GetRootPath()
            pid_file = root_path + "/conf/myt.pid"
            if os.path.isfile(pid_file):
                with open(pid_file,'r') as f:
                    pid = f.read()
                    if self.check_process(pid) == True:
                        ret =  True
                    else:
                        ret = False
            else:
                ret = False

            if ret == False:
                with open(pid_file, 'w') as f:
                    f.write(str(os.getpid()))
        return ret
    
    #判断进程是否存在
    def check_process(self, pid):
        try:
            process = psutil.Process(int(pid))
            return True
        except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
            return False
        