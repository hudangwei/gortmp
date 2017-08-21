package rtmp

type InvokeProc interface {
	OnInvokeProc(string, AmfData, *RtmpConn)
}
