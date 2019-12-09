package uvtime

import "time"

func GettimeMs() int64{
//如果抽象成这样:aeb
//	要求a不能不写,也就是说是1也要写上
//	b必须是整数.
//		实现上就是 a*10^b
//	a乘以10的b次方
//	所以楼主的就是1*10^6
	return time.Now().UnixNano() / 1e6
}

func GettimeS() int64{
	return time.Now().Unix()
}

func GettimeNs() int64{
	return time.Now().UnixNano()
}

