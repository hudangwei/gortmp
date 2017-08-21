package rtmp

import "fmt"
import "net"
import "time"
import "encoding/binary"
import "math/rand"
import "sync"

type RtmpConn struct {
	conn          net.Conn
	aryData       []byte
	chunkSize     int
	preBodySize   int
	preTimeStamp  int
	prePacketType int
	handshakeC0   bool
	handshakeC2   bool
	dataLock      sync.Mutex
	invokeHandler InvokeProc
	chunkStreamId byte
}

func (c *RtmpConn) Init(chunkSize int, conn net.Conn, invokeHandler InvokeProc) {
	c.handshakeC0 = false
	c.handshakeC2 = false
	c.conn = conn
	c.chunkSize = chunkSize
	c.invokeHandler = invokeHandler
}

func (c *RtmpConn) OnRecv(b []byte, dataLen int) {

	c.dataLock.Lock()

	defer c.dataLock.Unlock()

	c.aryData = append(c.aryData, b[:dataLen]...)

	if !c.handshakeC0 {
		if len(c.aryData) < 1537 {
			return
		}

		c.handshakeC0 = true
		if !c.ProcessHandshake() {
			fmt.Println("Processhandshake eror!")
			return
		}

		if len(c.aryData) > 1537 {
			c.aryData = c.aryData[1537:]
		} else {
			c.aryData = make([]byte, 0)
		}

	} else if !c.handshakeC2 {
		if len(c.aryData) < 1536 {
			return
		}

		c.handshakeC2 = true

		if len(c.aryData) > 1536 {
			c.aryData = c.aryData[1536:]
		} else {
			c.aryData = make([]byte, 0)
		}
	}

	if len(c.aryData) > 0 {

		for {
			packet, ret := c.DecodePacket()
			if ret {

				c.ProcessPacket(packet)

				packetLen := packet.GetPacketLen()
				c.aryData = c.aryData[packetLen:]

			} else {
				break
			}
		}
	}
}

func (c *RtmpConn) ProcessPacket(p RtmpPacket) {

	c.chunkStreamId = p.GetChunkStreamId()
	packetType := p.GetPacketType()
	switch packetType {
	case AMF_SET_CHUNKSIZE:
	case AMF_STREAM_BEGIN:
	case AMF_ACK_SIZE:
		c.ProcessAckSize()
	case AMF_BAND_WIDTH:
	case AMF_TYPE_AUDIO:
	case AMF_TYPE_VIDEO:
	case AMF_TYPE_NOTIFY:
	case AMF_TYPE_INVOKE:
		c.ProcessInvoke(p)
	}

}

func (c *RtmpConn) ProcessAckSize() {
	fmt.Printf("Process acknowledgement size\n")
}

func (c *RtmpConn) ProcessInvoke(p RtmpPacket) {
	var amfCommand Amf0
	strCommand := amfCommand.GetCommand(p.GetBodyData())
	if strCommand == "" {
		fmt.Printf("Command error!\n")
		return
	}

	var amf Amf0
	amfData := amf.ReadData(p.GetBodyData(), false)

	switch strCommand {
	case "connect":
		c.SendAckSize(p.GetChunkStreamId(), 2500000)
		c.SendSetPeerBandwidth(p.GetChunkStreamId(), 2500000)
		c.SendControlMessage(p.GetChunkStreamId(), 0)
		c.SendResultMsg(p)
		c.SendOnBwDoneMsg(p)
	default:
		c.invokeHandler.OnInvokeProc(strCommand, amfData, c)
	}
}

func (c *RtmpConn) OnError(err error) {
}

func (c *RtmpConn) DecodePacket() (RtmpPacket, bool) {
	var packet RtmpPacket
	ret := packet.Decode(c.aryData, len(c.aryData), c.chunkSize, c.preBodySize, c.preTimeStamp, c.prePacketType)
	if !ret {
		//fmt.Printf("Decode packet error.")
		return packet, false
	}

	c.preBodySize = packet.GetBodySize()
	c.preTimeStamp = packet.GetTimeStamp()
	c.prePacketType = packet.GetPacketType()

	return packet, true
}

func (c *RtmpConn) ProcessHandshake() bool {
	if c.aryData[0] != 3 {
		return false
	}

	c.SendHandshakeS0S1S2()

	return true
}

func (c *RtmpConn) SendHandshakeC0C1() bool {

	handshakeData := make([]byte, 1537)
	handshakeData[0] = 0x03

	ts := time.Now()

	tsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(tsBuf, uint64(ts.Unix()))

	for i := 0; i < 8; i++ {
		//fmt.Printf("%02X", tsBuf[i])
		handshakeData[i+1] = tsBuf[i]
	}

	for i := 9; i < 1537; i++ {
		handshakeData[i] = byte(rand.Intn(256))
	}

	return true
}

func (c *RtmpConn) SendHandshakeS0S1S2() bool {

	var hb HandShakeBuf
	hb.InitBuf()

	n, err := c.conn.Write(hb.GetBuf())

	if err != nil {
		fmt.Println("write s0s1s2 error!")
		return false
	}

	fmt.Printf("write s0s1s2 OK. n=%d\n", n)

	return true
}

func (c *RtmpConn) SendAckSize(chunkStreamId byte, ackSize uint32) bool {
	var packet RtmpPacket
	ackData := packet.AckPacket(chunkStreamId, ackSize)
	_, err := c.conn.Write(ackData)
	if err != nil {
		fmt.Println("write s0s1s2 error!")
		return false
	}

	return true
}

func (c *RtmpConn) SendSetPeerBandwidth(chunkStreamId byte, bandwidthSize uint32) bool {
	var packet RtmpPacket
	bnadwidthData := packet.SetPeerBandwidthPacket(chunkStreamId, bandwidthSize)
	_, err := c.conn.Write(bnadwidthData)
	if err != nil {
		fmt.Println("write s0s1s2 error!")
		return false
	}

	return true
}

func (c *RtmpConn) SendControlMessage(chunkStreamId byte, eventType uint16) bool {
	var packet RtmpPacket
	controlMsgData := packet.ControlMessagePacket(chunkStreamId, eventType)
	_, err := c.conn.Write(controlMsgData)
	if err != nil {
		fmt.Println("write conctrol message error!")
		return false
	}

	return true
}

func (c *RtmpConn) SendInvokeMessage(headerType byte, timeStamp int, bodyData []byte) bool {
	var packet RtmpPacket
	invokeData := packet.InvokeMessage(headerType, c.chunkStreamId, timeStamp, bodyData, c.chunkSize)

	_, err := c.conn.Write(invokeData)
	if err != nil {
		fmt.Println("write invoke message error!")
		return false
	}

	return true
}

func (c *RtmpConn) SendResultMsg(p RtmpPacket) {

	var amfObj Amf0
	amfObj.WriteString("_result")
	amfObj.WriteNumber(1)
	amfObj.WriteObjectBegin()
	amfObj.WritePropertyString("fmsVer", "FMS/3,0,1,123")
	amfObj.WritePropertyNumber("capabilities", 31)
	amfObj.WriteObjectEnd()

	amfObj.WriteObjectBegin()
	amfObj.WritePropertyString("level", "status")
	amfObj.WritePropertyString("code", "NetConnection.Connect.Success")
	amfObj.WritePropertyString("description", "Connection succeeded")
	amfObj.WritePropertyNumber("objectEncoding", 0)
	amfObj.WriteObjectEnd()

	/*
		fmt.Println("OBJ--------------")
		for i := 0; i < len(amfObj.GetData()); i++ {
			//fmt.Printf("bodylen=%d, \nbodyData=:\n%02X\n", len(amfObj.GetData()), amfObj.GetData())
			if i%16 == 0 {
				fmt.Printf("\n")
			}
			fmt.Printf("%02X ", amfObj.GetData()[i])
		}
		//fmt.Printf("bodylen=%d, \nbodyData=:\n%02X\n", len(amfObj.GetData()), amfObj.GetData())
	*/

	c.SendInvokeMessage(byte(PACKET_FMT_12), 0, amfObj.GetData())
}

func (c *RtmpConn) SendOnBwDoneMsg(p RtmpPacket) {
	var amfObj Amf0
	amfObj.WriteString("onBWDone")
	amfObj.WriteNumber(0)
	amfObj.WriteNull()
	amfObj.WriteNumber(8192)

	c.SendInvokeMessage(byte(PACKET_FMT_8), 0, amfObj.GetData())
}

func (c *RtmpConn) GetConn() net.Conn {
	return c.conn
}
