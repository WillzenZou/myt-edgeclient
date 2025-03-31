// 下列 ifdef 块是创建使从 DLL 导出更简单的
// 宏的标准方法。此 DLL 中的所有文件都是用命令行上定义的 LIBMYTRPC_EXPORTS
// 符号编译的。在使用此 DLL 的
// 任何其他项目上不应定义此符号。这样，源文件中包含此文件的任何其他项目都会将
// LIBMYTRPC_API 函数视为是从 DLL 导入的，而此 DLL 则将用此宏定义的
// 符号视为是被导出的。
#ifdef WIN32
	#ifdef LIBMYTRPC_EXPORTS
	#define LIBMYTRPC_API __declspec(dllexport)
	#else
	#define LIBMYTRPC_API __declspec(dllimport)
	#endif
#else
	#define LIBMYTRPC_API
	#define WINAPI
	typedef unsigned char BYTE;
#endif

#define  MYAPI __cdecl

typedef long long DLONG;

#ifdef __cplusplus
extern "C"{
#endif
/*
函数说明:获取当前库的版本号
*/
LIBMYTRPC_API int  MYAPI  getVersion();

/*
函数说明:远程连接设备
参数说明:
	ip : 需要远程控制的设备的IP地址
	port :需要远程控制的设备的端口
	timeout: 远程连接的超时时间单位是秒
返回值:返回一个长整形id，后面所有的操作函数都要用这个id 大于0表示成功失败返回0
*/
LIBMYTRPC_API long  MYAPI   openDevice(const char* ip,int port,long timeout);
/*
函数说明:关闭远程连接
参数说明:
	handle : openDevice返回的id
返回值:大于0表示成功失败返回0
*/
LIBMYTRPC_API int   MYAPI   closeDevice(long handle);

/*
函数说明:检查远程连接是否处于连接状态
参数说明:
	handle : openDevice返回的id
返回值:1表示连接中0表示已断开
*/
LIBMYTRPC_API int MYAPI checkLive(long handle);
/*
函数说明:远程截图，获取的是RGBA像素数组
参数说明:
	handle : openDevice返回的id
	w:		指针类型返回图片的宽度
	h:		指针类型返回图片的高度
返回值:指针类型的像素数组（释放用freeRpcPtr）
*/
LIBMYTRPC_API BYTE* MYAPI   takeCaptrue(long handle,int* w,int* h,int* stride);
LIBMYTRPC_API BYTE* MYAPI   takeCaptrueEx(long handle,int l,int t,int r,int b,int* w,int* h,int* stride);
/*
函数说明:远程截图，获取的是压缩的png或者jpg类型的文件流
参数说明:
	handle : openDevice返回的id
	type:	 0表示获取png 1表示获取jpg
	quality: 表示压缩质量取值是0-100
	len:指针类型表示接受数据流的大小
返回值:指针类型的数据流（释放用freeRpcPtr）
*/
LIBMYTRPC_API BYTE* MYAPI	 takeCaptrueCompress(long handle,int type,int quality,int* len);
LIBMYTRPC_API BYTE* MYAPI	 takeCaptrueCompressEx(long handle,int l,int t,int r,int b,int type,int quality,int* len);
/*
函数说明:模拟按下
参数说明:
	handle : openDevice返回的id
	id:	 表示要按下的手指编号（0-9）
	x: 水平坐标
	y: 垂直坐标
*/
LIBMYTRPC_API int   MYAPI	 touchDown(long handle,int id,int x,int y);
/*
函数说明:模拟弹起
参数说明:
	handle : openDevice返回的id
	id:	 表示要弹起的手指编号（0-9）
	x: 水平坐标
	y: 垂直坐标
*/
LIBMYTRPC_API int   MYAPI	 touchUp(long handle,int id,int x,int y);
/*
函数说明:模拟滑动
参数说明:
	handle : openDevice返回的id
	id:	 表示要按下的手指编号（0-9）
	x: 水平坐标
	y: 垂直坐标
*/
LIBMYTRPC_API int   MYAPI	 touchMove(long handle,int id,int x,int y);


/*
函数说明:模拟滑动
参数说明:
	handle : openDevice返回的id
	id:	 表示要按下的手指编号（0-9）
	x0,y0: 初始坐标
	x1,y1: 结束坐标
	millis: 动作完成时间
*/
LIBMYTRPC_API void MYAPI swipe(long handle,int id,int x0, int y0, int x1, int y1, long millis,bool async);

/*
函数说明:模拟单击
参数说明:
	handle : openDevice返回的id
	id:	 表示要单击的手指编号（0-9）
	x: 水平坐标
	y: 垂直坐标
*/
LIBMYTRPC_API int   MYAPI	 touchClick(long handle,int id,int x,int y);
/*
函数说明:模拟按键
参数说明:
	handle : openDevice返回的id
	code:	 表示按键码（自行查看KeyEvent.java的按键码）
*/
LIBMYTRPC_API int   MYAPI	 keyPress(long handle,int code);
/*
函数说明:模拟文字输入
参数说明:
	handle : openDevice返回的id
	text:	要输入的字符串
*/
LIBMYTRPC_API int   MYAPI	 sendText(long handle,const char* text);
/*
函数说明:打开指定包名的app
参数说明:
	handle : openDevice返回的id
	pkg:	包名
*/
LIBMYTRPC_API int   MYAPI	 openApp(long handle,const char* pkg);
/*
函数说明:关闭指定包名的app
参数说明:
	handle : openDevice返回的id
	pkg:	包名
*/
LIBMYTRPC_API int   MYAPI	 stopApp(long handle,const char* pkg);
/*
函数说明:释放截图数据（截图数据必须用这个函数释放否则会造成内存泄露）
参数说明:
	handle : openDevice返回的id
	ptr:	指针类型
*/
LIBMYTRPC_API void  MYAPI   freeRpcPtr(void* ptr);

/*
函数说明:导出节点数据（返回的指针必须用freeRpcPtr释放否则会造成内存泄露）
参数说明:
	handle : openDevice返回的id
	ptr:	指针类型
*/
LIBMYTRPC_API char* MYAPI dumpNodeXml(long handle,int bDumpAll);

/*
函数说明:获取当前屏幕旋转
参数说明:
	handle : openDevice返回的id
返回结果:整数（0,1,2,3）
*/
LIBMYTRPC_API int MYAPI getDisplayRotate(long handle);

/*
函数说明:执行shell命令（返回的指针必须用freeRpcPtr释放否则会造成内存泄露）
参数说明:
	handle : execCmd返回执行的结果
	bWaitForExit:	是否等待执行结束才返回
	cmdline: 命令行
*/
LIBMYTRPC_API char* MYAPI execCmd(long handle,int bWaitForExit,const char* cmdline);

/*
函数说明:开启屏幕镜像拉流(输出的是h264数据)
参数说明:
	handle :openDevice返回的id
	w,h:	希望输出的尺寸长和宽
	bitrate 输出的比特率 
	cb: 接收屏幕镜像流的回调函数（rot 是当前屏幕方向0,1,2,3 对应0 90 180 270度）
	audio_cb :接收系统内部声音数据流
*/
LIBMYTRPC_API int MYAPI startVideoStream(long handle,int w,int h,int bitrate, void (*cb)(int rot,void* data,int len),void(*audio_cb)(void*, int));

/*
函数说明:关闭屏幕镜像流
参数说明:
	handle : openDevice返回的id
*/
LIBMYTRPC_API int MYAPI stopVideoStream(long handle);

/**********************************************************************/
/**********************************************************************/
/**********************************************************************/
/**********************************************************************/
/*
			针对节点封装的方法集合
	Demo:
			//连接设备
			long handle = openDevice("192.168.30.2",11020,3);
			if(handle){
				//创建筛选器  中文只支持UTF8 编码
				long sel = newSelector(handle);


				//添加筛选器的查询条件  可以添加多个条件
				std::string item = GBKToUTF8("文件");
				std::string clzName = "clsTest"
				TextContainWith(sel,item.c_str());		
				BoundsInside(sel,0,0,720,1280);
				ClzContainWith(sel, clzName);
				Checkable(sel, true);
			
				//按照设置的条件进行查找并返回结果集
				long nodes = findNodes(sel,10,5000);
				long size = getNodesSize(nodes);
				if(size > 0){
					//对符合要求的结果集进行处理
					for(int i = 0;i<size;i++){
						long node = getNodeByIndex(nodes,i);
						
						//点击节点
						clickNode(node);		

						//获取节点其他的属性信息
						long p = getNodeParent(node);
						char* text = getNodeText(node);
						if(text && strlen(text) > 0)
						   printf("%s\n",text);
						if(text) freeRpcPtr(text);
						
					}
				}
				freeNodes(nodes);
				freeSelector(sel);
				closeDevice(handle);
			}

*/
/**********************************************************************/
/**********************************************************************/
/**********************************************************************/
/**********************************************************************/
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/*
函数说明:创建一个筛选器
参数说明:
	handle : openDevice返回的id
返回值:返回一个筛选器唯一表示句柄
*/
LIBMYTRPC_API DLONG  newSelector(DLONG handle);

/*
函数说明:清除筛选器里面的所有的筛选条件
参数说明:
	sel : newSelector创建的筛选器句柄
*/
LIBMYTRPC_API void clearSelector(DLONG sel);

/*
函数说明:释放筛选器
参数说明:
	sel : newSelector创建的筛选器句柄
*/
LIBMYTRPC_API void  freeSelector(DLONG sel);

/*
函数说明:根据帅选器去查找
参数说明:
	sel : newSelector创建的筛选器句柄
	maxCntRet:期望最多筛选出maxCntRet个结果，超过这个值直接返回
	timeout:查找超时时间，没有筛选到期待的结果会一直查找直到超时否则如果找到就直接返回
返回值:返回一个结果集的唯一标识句柄
*/
LIBMYTRPC_API DLONG  findNodes(DLONG sel,int maxCntRet,int timeout);

/*
函数说明:释放结果集
参数说明:
	nodes : findNodes返回的结果集
*/
LIBMYTRPC_API void  freeNodes(DLONG nodes);


/**********************************************************************/
// 以下方法均为 对节点的属性操作方法
/**********************************************************************/

/*
函数说明:获取结果集中节点的个数
参数说明:
	nodes : findNodes返回的结果集
返回值:结果集中节点的个数
*/
LIBMYTRPC_API DLONG  getNodesSize(DLONG nodes);

/*
函数说明:根据顺序索引获取结果集中的节点
参数说明:
	nodes: findNodes返回的结果集
	index: 结果集是一个数组，这个index就是要获取节点的数组下标
返回值:返回一个节点对象的唯一标识句柄
*/
LIBMYTRPC_API DLONG  getNodeByIndex(DLONG nodes,int index);

/*
函数说明:获取给定节点的父节点的句柄
参数说明:
	node:节点句柄
返回值:返回这个节点的父节点的句柄
*/
LIBMYTRPC_API DLONG  getNodeParent(DLONG node);

/*
函数说明:获取给定节点的子节点个数
参数说明:
	node:节点句柄
返回值:节点的子节点个数
*/
LIBMYTRPC_API DLONG  getNodeChildCount(DLONG node);

/*
函数说明:获取给定节点的子节点集
参数说明:
	node:节点句柄
返回值:给定节点的子节点集句柄
*/
LIBMYTRPC_API DLONG  getNodeChild(DLONG node,int index);

/*
函数说明:获取给定节点的json字符串形式
参数说明:
	node:节点句柄
返回值:给定节点的json字符串
*/
LIBMYTRPC_API char* getNodeJson(DLONG node);

/*
函数说明:获取给定节点的文本属性
参数说明:
	node:节点句柄
返回值:给定节点的节点的文本属性字符串
*/
LIBMYTRPC_API char* getNodeText(DLONG node);

/*
函数说明:获取给定节点的描述属性
参数说明:
	node:节点句柄
返回值:给定节点的节点的描述属性字符串
*/
LIBMYTRPC_API char* getNodeDesc(DLONG node);

/*
函数说明:获取给定节点的包名属性
参数说明:
	node:节点句柄
返回值:给定节点的节点的包名属性字符串
*/
LIBMYTRPC_API char* getNodePackage(DLONG node);

/*
函数说明:获取给定节点的类名属性
参数说明:
	node:节点句柄
返回值:给定节点的节点的类名属性字符串
*/
LIBMYTRPC_API char* getNodeClass(DLONG node);

/*
函数说明:获取给定节点的资源ID属性
参数说明:
	node:节点句柄
返回值:给定节点的节点的资源ID属性字符串
*/
LIBMYTRPC_API char* getNodeId(DLONG node);

/*
函数说明:获取给定节点的范围属性
参数说明:
	node:节点句柄
	l,t,r,b 是整型指针类型用于接收节点的上下左右范围参数
返回值:整型1表示成功0表示失败
*/
LIBMYTRPC_API int getNodeNound(DLONG node,int* l,int* t,int* r,int* b);

/*
函数说明:获取给定节点中心坐标
参数说明:
	node:节点句柄
	x,y 是整型指针类型用于接收节点中心的横纵坐标
返回值:整型1表示成功0表示失败
*/
LIBMYTRPC_API int getNodeNoundCenter(DLONG node,int* x,int* y);

/**********************************************************************/
// 以下方法均为 对节点的操作响应事件
/**********************************************************************/
/*
函数说明:点击节点
参数说明:
	node:节点句柄
返回值:整型1表示成功0表示失败
*/
LIBMYTRPC_API int  clickNode(DLONG node);

/*
函数说明:长按节点
参数说明:
	node:节点句柄
返回值:整型1表示成功0表示失败
*/
LIBMYTRPC_API int longClickNode(DLONG node);



/**********************************************************************/
// 以下方法均为筛选器的属性 针对筛选器可以多次叠加
// 注意: 删选出结果后必须调用 freeSelector  释放筛选器
/**********************************************************************/
/*
函数说明:设置节点是否可用筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Enable(DLONG sel, bool v);

/*
函数说明:设置节点是否可以被选中筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Checkable(DLONG sel, bool v);

/*
函数说明:设置节点是否可以被点击筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Clickable(DLONG sel, bool v);

/*
函数说明:设置节点是否可以获取焦点筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Focusable(DLONG sel, bool v);

/*
函数说明:设置节点是否已经获取焦点筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Focused(DLONG sel, bool v);

/*
函数说明:设置节点是否可以滚动筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Scrollable(DLONG sel, bool v);

/*
函数说明:设置节点是否可以长点击筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void LongClickable(DLONG sel, bool v);

/*
函数说明:设置节点是否是密码框筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Password(DLONG sel, bool v);

/*
函数说明:设置节点是否可选择筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Selected(DLONG sel, bool v);

/*
函数说明:设置节点是否可见筛选器
参数说明:
	node:节点句柄
	v:布尔类型
*/
LIBMYTRPC_API void Visible(DLONG sel, bool v);

/*
函数说明:设置节点索引筛选器
参数说明:
	node:节点句柄
	v:整数类型
*/
LIBMYTRPC_API void Index(DLONG sel, int v);

/*
函数说明:设置在节点指定范围内的筛选器
参数说明:
	node:节点句柄
	l,t,r,b:整数类型表示上下左右范围
*/
LIBMYTRPC_API void BoundsInside(DLONG sel, int l,int t,int r,int b);

/*
函数说明:设置节点的范围等于指定范围的筛选器
参数说明:
	node:节点句柄
	l,t,r,b:整数类型表示上下左右范围
*/
LIBMYTRPC_API void BoundsEqual(DLONG sel, int l,int t,int r,int b);

/*
函数说明:设置节点的资源ID等于设置的id的筛选器
参数说明:
	node:节点句柄
	str:指定资源id
*/
LIBMYTRPC_API void IdEqual(DLONG sel, const char* str);

/*
函数说明:设置节点的资源ID以指定字符串开头的筛选器
参数说明:
	node:节点句柄
	str:字符串
*/
LIBMYTRPC_API void IdStartWith(DLONG sel, const char* str);

/*
函数说明:设置节点的资源ID以指定字符串结尾的筛选器
参数说明:
	node:节点句柄
	str:字符串
*/
LIBMYTRPC_API void IdEndWith(DLONG sel, const char* str);

/*
函数说明:设置节点的资源ID包含指定字符串的筛选器
参数说明:
	node:节点句柄
	str:字符串
*/
LIBMYTRPC_API void IdContainWith(DLONG sel, const char* str);

/*
函数说明:设置节点的资源ID能正则匹配指定字符串的筛选器
参数说明:
	node:节点句柄
	str:字符串
*/
LIBMYTRPC_API void IdMatchWith(DLONG sel, const char* str);

LIBMYTRPC_API void TextStartWith(DLONG sel, const char* str);

LIBMYTRPC_API void TextEndWith(DLONG sel, const char* str);

LIBMYTRPC_API void TextContainWith(DLONG sel, const char* str);

LIBMYTRPC_API void TextMatchWith(DLONG sel, const char* str);

LIBMYTRPC_API void ClzStartWith(DLONG sel, const char* str);

LIBMYTRPC_API void ClzEndWith(DLONG sel, const char* str);

LIBMYTRPC_API void ClzContainWith(DLONG sel, const char* str);

LIBMYTRPC_API void ClzMatchWith(DLONG sel, const char* str);

LIBMYTRPC_API void PackageStartWith(DLONG sel, const char* str);

LIBMYTRPC_API void PackageEndWith(DLONG sel, const char* str);

LIBMYTRPC_API void PackageContainWith(DLONG sel, const char* str);

LIBMYTRPC_API void PackageMatchWith(DLONG sel, const char* str);

LIBMYTRPC_API void DescStartWith(DLONG sel, const char* str);

LIBMYTRPC_API void DescEndWith(DLONG sel, const char* str);

LIBMYTRPC_API void DescContainWith(DLONG sel, const char* str);

LIBMYTRPC_API void DescMatchWith(DLONG sel, const char* str);

LIBMYTRPC_API int MYAPI useNewNodeMode(long handle, bool use);

LIBMYTRPC_API char* MYAPI dumpNodeXmlEx(long handle, int useNewMode, int timeout);

#ifdef __cplusplus
}
#endif
