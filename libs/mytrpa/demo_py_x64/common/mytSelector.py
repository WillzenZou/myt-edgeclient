import ctypes
from common.rpcNode import rpcNode
class mytSelector(object):    
    def __init__(self,  rpc_handle, rpc_dll) -> None:
        self._rpc_dll = rpc_dll
        self._rpc_dll.newSelector.restype = ctypes.c_longlong
        self._selhande = rpc_dll.newSelector(rpc_handle)
        self._nodeHandle = 0

    def __del__(self):
        
        if self._nodeHandle != 0:
            self._rpc_dll.freeNodes.argtypes = [ctypes.c_longlong]
            self._rpc_dll.freeNodes(self._nodeHandle)

        if self._selhande != 0:
            self._rpc_dll.freeSelector.argtypes = [ctypes.c_longlong]
            self._rpc_dll.freeSelector(self._selhande)
        


    #匹配with 方式使用
    def __enter__(self):
        pass
    def __exit__(self,type,value,trace):
        pass
    
    

    #执行查询返回符合条件的Node结果集
    #	maxCntRet:期望最多筛选出maxCntRet个结果，超过这个值直接返回
	#   timeout:查找超时时间，没有筛选到期待的结果会一直查找直到超时否则如果找到就直接返回  单位:[毫秒]
    # 返回Node节点数组
    def execQuery(self, maxNode, timeout, reset_query = True):
        node_arr = []
        if self._selhande != 0:

            self._rpc_dll.findNodes.restype = ctypes.c_longlong
            self._rpc_dll.findNodes.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_int]
            self._rpc_dll.getNodesSize.restype = ctypes.c_longlong
            self._rpc_dll.getNodesSize.argtypes = [ctypes.c_longlong]
            self._rpc_dll.getNodeByIndex.restype = ctypes.c_longlong
            self._rpc_dll.getNodeByIndex.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._nodeHandle = self._rpc_dll.findNodes(self._selhande, maxNode, timeout)
            if self._nodeHandle != 0:
                nodeSize = self._rpc_dll.getNodesSize(self._nodeHandle)
                for i in range(0, nodeSize, 1):
                    node_hande = self._rpc_dll.getNodeByIndex(self._nodeHandle, i)
                    new_node = rpcNode(node_hande, self._rpc_dll)
                    node_arr.append(new_node)    

            if reset_query == True:
                self._rpc_dll.clearSelector.argtypes = [ctypes.c_longlong]
                self._rpc_dll.clearSelector(self._selhande)         
                   
        return node_arr
    
    def execQueryOne(self, timeout, reset_query = True):
        ret = None
        if self._selhande != 0:
            self._rpc_dll.findNodes.restype = ctypes.c_longlong
            self._rpc_dll.findNodes.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_int]
            self._rpc_dll.getNodesSize.restype = ctypes.c_longlong
            self._rpc_dll.getNodesSize.argtypes = [ctypes.c_longlong]
            self._rpc_dll.getNodeByIndex.restype = ctypes.c_longlong
            self._rpc_dll.getNodeByIndex.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._nodeHandle = self._rpc_dll.findNodes(self._selhande, 1, timeout)
            if self._nodeHandle != 0:
                nodeSize = self._rpc_dll.getNodesSize(self._nodeHandle)
                if nodeSize>0:
                    node_hande = self._rpc_dll.getNodeByIndex(self._nodeHandle, 0)
                    ret = rpcNode(node_hande, self._rpc_dll)
            
            if reset_query == True:
                self._rpc_dll.clearSelector.argtypes = [ctypes.c_longlong]
                self._rpc_dll.clearSelector(self._selhande)

        return ret

    #清除查询条件
    def clear_Query(self):
        if self._selhande != 0:
            self._rpc_dll.clearSelector.argtypes = [ctypes.c_longlong]
            self._rpc_dll.clearSelector(self._selhande)

    #添加查询条件
    def addQuery_Enable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Enable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Enable(self._selhande, enable)

    def addQuery_Checkable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Checkable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Checkable(self._selhande, enable)

    def addQuery_Clickable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Clickable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Clickable(self._selhande, enable)

    def addQuery_Focusable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Focusable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Focusable(self._selhande, enable)

    #已经获取焦点
    def addQuery_Foucesd(self, val):
        if self._selhande!=0:
            self._rpc_dll.Focused.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Focused(self._selhande, val)

    def addQuery_Scrollable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Scrollable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Scrollable(self._selhande, enable)
    
    def addQuery_LongClickable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.LongClickable.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.LongClickable(self._selhande, enable) 

    def addQuery_Passwordable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Password.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Password(self._selhande, enable)  

    def addQuery_Selectedable(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Selected.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Selected(self._selhande, enable)     

    def addQuery_Visible(self, enable):
        if self._selhande!=0:
            self._rpc_dll.Visible.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Visible(self._selhande, enable)   

    def addQuery_index(self, ids):
        if self._selhande!=0:
            self._rpc_dll.Index.argtypes = [ctypes.c_longlong, ctypes.c_int]
            self._rpc_dll.Index(self._selhande, ids)     

    def addQuery_BoundsInside(self, left, top, right, bottom):
        if self._selhande!=0:
            self._rpc_dll.BoundsInside.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_int]
            self._rpc_dll.BoundsInside(self._selhande, left, top, right, bottom)    

    def addQuery_BoundsEqual(self, left, top, right, bottom):
        if self._selhande!=0:
            self._rpc_dll.BoundsEqual.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_int]
            self._rpc_dll.BoundsEqual(self._selhande, left, top, right, bottom)  

    def addQuery_IdEqual(self, str_id):
        if self._selhande!=0:
            self._rpc_dll.IdEqual.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.IdEqual(self._selhande, ctypes.c_char_p(str_id.encode('utf-8')))  
    
    def addQuery_IdStartWith(self, str_id):
        if self._selhande!=0:
            self._rpc_dll.IdStartWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.IdStartWith(self._selhande, ctypes.c_char_p(str_id.encode('utf-8')))  

    def addQuery_IdEndWith(self, str_id):
        if self._selhande!=0:
            self._rpc_dll.IdEndWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.IdEndWith(self._selhande, ctypes.c_char_p(str_id.encode('utf-8')))  

    def addQuery_IdContainWith(self, str_id):
        if self._selhande!=0:
            self._rpc_dll.IdContainWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.IdContainWith(self._selhande, ctypes.c_char_p(str_id.encode('utf-8')))  

    def addQuery_IdMatchWith(self, str_id):
        if self._selhande!=0:
            self._rpc_dll.IdMatchWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.IdMatchWith(self._selhande, ctypes.c_char_p(str_id.encode('utf-8')))  


    def addQuery_TextEqual(self, text):
        if self._selhande!=0:
            self._rpc_dll.TextEqual.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.TextEqual(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  
    
    def addQuery_TextStartWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.TextStartWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.TextStartWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_TextEndWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.TextEndWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.TextEndWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_TextContainWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.TextContainWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.TextContainWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_TextMatchWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.TextMatchWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.TextMatchWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  



    def addQuery_ClzEqual(self, text):
        if self._selhande!=0:
            self._rpc_dll.ClzEqual.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.ClzEqual(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  
    
    def addQuery_ClzStartWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.ClzStartWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.ClzStartWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_ClzEndWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.ClzEndWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.ClzEndWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_ClzContainWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.ClzContainWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.ClzContainWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_ClzMatchWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.ClzMatchWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.ClzMatchWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  


    def addQuery_PackageEqual(self, text):
        if self._selhande!=0:
            self._rpc_dll.PackageEqual.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.PackageEqual(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  
    
    def addQuery_PackageStartWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.PackageStartWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.PackageStartWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_PackageEndWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.PackageEndWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.PackageEndWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_PackageContainWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.PackageContainWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.PackageContainWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_PackageMatchWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.PackageMatchWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.PackageMatchWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  



    def addQuery_DescEqual(self, text):
        if self._selhande!=0:
            self._rpc_dll.DescEqual.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.DescEqual(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  
    
    def addQuery_DescStartWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.DescStartWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.DescStartWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_DescEndWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.DescEndWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.DescEndWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_DescContainWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.DescContainWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.DescContainWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  

    def addQuery_DescMatchWith(self, text):
        if self._selhande!=0:
            self._rpc_dll.DescMatchWith.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
            self._rpc_dll.DescMatchWith(self._selhande, ctypes.c_char_p(text.encode('utf-8')))  
