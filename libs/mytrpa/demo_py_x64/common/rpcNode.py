import ctypes
class rpcNode(object):
    def __init__(self, handle, rpc_dll):
        self._handle = handle
        self._rpc = rpc_dll

    def getParent(self):
        ret = None
        if self._handle!=0:
            self._rpc.getNodeParent.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeParent.restype = ctypes.c_longlong
            node_handle = self._rpc.getNodeParent(self._handle)
            if node_handle>0:
                ret = rpcNode(node_handle, self._rpc)
        return ret
    
    #获取孩子节点的数量
    def getChildCount(self):
        ret = 0
        if self._handle!=0:
            self._rpc.getNodeChildCount.argtypes = [ctypes.c_longlong]
            ret = self._rpc.getNodeChildCount(self._handle)
        return ret
    
    #获取孩子节点数组
    def getChild(self):
        ret = []
        if self._handle!=0:
            self._rpc.getNodeChildCount.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeChild.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc.getNodeChild.restype = ctypes.c_longlong
            childnum = self._rpc.getNodeChildCount(self._handle)
            for i in range(0, childnum, 1):
                node_handle = self._rpc.getNodeChild(self._handle, i)
                new_node = rpcNode(node_handle, self._rpc)
                ret.append(new_node)
        return ret
    
    #获取节点的json数据
    def getNodeJson(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodeJson.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeJson.restype = ctypes.c_void_p
            ptr = self._rpc.getNodeJson(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret

    #获取给定节点的文本属性
    def getNodeText(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodeText.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeText.restype = ctypes.c_void_p
            ptr = self._rpc.getNodeText(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret

    #获取给定节点的文本属性
    def getNodeDesc(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodeDesc.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeDesc.restype = ctypes.c_void_p
            ptr = self._rpc.getNodeDesc(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret
    
        #获取给定节点的包名属性
    def getNodePackage(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodePackage.argtypes = [ctypes.c_longlong]
            self._rpc.getNodePackage.restype = ctypes.c_void_p
            ptr = self._rpc.getNodePackage(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret

        #获取给定节点的类名属性
    def getNodeClass(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodeClass.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeClass.restype = ctypes.c_void_p
            ptr = self._rpc.getNodeClass(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret

    #获取给定节点的资源ID属性
    def getNodeId(self):
        ret = ''
        if self._handle!=0:
            self._rpc.getNodeId.argtypes = [ctypes.c_longlong]
            self._rpc.getNodeId.restype = ctypes.c_void_p
            ptr = self._rpc.getNodeId(self._handle)
            p2 = ctypes.cast(ptr, ctypes.c_char_p)
            ret = p2.value.decode("utf-8")
            self._rpc.freeRpcPtr.argtypes = [ctypes.c_void_p]
            self._rpc.freeRpcPtr(ptr)
        return ret

#     #获取给定节点的范围属性
#     函数说明:获取给定节点的范围属性
# 参数说明:
# 	node:节点句柄
# 	l,t,r,b 是整型指针类型用于接收节点的上下左右范围参数
# 返回值:整型1表示成功0表示失败
    def getNodeNound(self):
        ret = {'left':-1,'top':-1, 'right':-1, 'bottom':-1}
        if self._handle!=0:
            self._rpc.getNodeNound.argtypes = [ctypes.c_longlong, ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int)]
            self._rpc.getNodeNound.restype = ctypes.c_int
            l = ctypes.c_int(0)
            t = ctypes.c_int(0)
            r = ctypes.c_int(0)
            b = ctypes.c_int(0)
            exec_ret = self._rpc.getNodeNound(self._handle, l,t,r,b)
            if exec_ret == 1:
                ret['left'] = l.value
                ret['top'] = t.value
                ret['right'] = r.value
                ret['bottom']  = b.value
        return ret

# 函数说明:获取给定节点中心坐标
# 参数说明:
# 	node:节点句柄
# 	x,y 是整型指针类型用于接收节点中心的横纵坐标
# 返回值:整型1表示成功0表示失败
    def getNodeNoundCenter(self):
        ret = {'x':-1, 'y':-1}
        if self._handle!=0:
            self._rpc.getNodeNoundCenter.argtypes = [ctypes.c_longlong, ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int)]
            self._rpc.getNodeNoundCenter.restype = ctypes.c_int
            x = ctypes.c_int(0)
            y = ctypes.c_int(0)
            exec_ret = self._rpc.getNodeNoundCenter(self._handle, x, y)
            if exec_ret == 1:
                ret['x'] = x.value
                ret['y'] = y.value
        return ret


    #点击当前节点
    def Click_events(self):
        ret = False
        if self._handle != 0:
            self._rpc.clickNode.argtypes = [ctypes.c_longlong]
            exec_ret = self._rpc.clickNode(self._handle)
            if exec_ret == 1 :
                ret = True
        return ret
    
    #长点击当前节点
    def longClick_events(self):
        ret = False
        if self._handle != 0:
            self._rpc.longClickNode.argtypes = [ctypes.c_longlong]
            exec_ret = self._rpc.longClickNode(self._handle)
            if exec_ret == 1 :
                ret = True
        return ret
