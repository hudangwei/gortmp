package rtmp

import (
	"bytes"
	"fmt"
	"io"
)

import "encoding/binary"

const (
	PACKET_FMT_12 = 0
	PACKET_FMT_8  = 1
	PACKET_FMT_4  = 2
	PACKET_FMT_1  = 3
)

const (
	AMF_SET_CHUNKSIZE = 0x01
	AMF_STREAM_BEGIN  = 0x04
	AMF_ACK_SIZE      = 0x05
	AMF_BAND_WIDTH    = 0x06
	AMF_TYPE_AUDIO    = 0x08
	AMF_TYPE_VIDEO    = 0x09
	AMF_TYPE_NOTIFY   = 0x12
	AMF_TYPE_INVOKE   = 0x14
)

type RtmpPacket struct {
	timeStamp     int
	bodySize      int
	headerLen     int
	hasExtendedTs bool
	packetType    int
	bodyData      []byte
	packetLen     int
	extTimestamp  int
	chunkStreamId byte
}

func (r *RtmpPacket) Decode(b []byte, dataLen int, chunkSize int, preBodySize int, preTimeStamp int, prePacketType int) bool {
	if dataLen < 1 {
		return false
	}

	headerType := (b[0] & 0xC0) >> 6

	headerLen, ret := r.checkEnoughHeader(int(headerType), dataLen)
	if !ret {
		return false
	}

	//fmt.Println("headertype=", headerType)

	r.chunkStreamId = byte(b[0] & 0x3F)
	r.timeStamp = 0
	r.bodySize = 0
	r.headerLen = headerLen
	r.hasExtendedTs = false
	r.packetType = 0

	if headerType == PACKET_FMT_12 || headerType == PACKET_FMT_8 || headerType == PACKET_FMT_4 {

		timeValueAry := []byte{0, b[1], b[2], b[3]}
		//fmt.Printf("bodysizeary=%v", timeValueAry)
		timeTs := r.bytes2Int(timeValueAry)

		if timeTs == 0xFFFFFF {
			r.hasExtendedTs = true
			if headerLen+4 > dataLen {
				fmt.Println("find extended timestamp, but data leng not enough")
				return false
			}

			headerLen += 4

			timeExtValueAry := []byte{b[headerLen-4], b[headerLen-3], b[headerLen-2], b[headerLen-1]}
			extendedTimeStamp := r.bytes2Int(timeExtValueAry)

			r.extTimestamp = extendedTimeStamp
		}
	}

	if headerType == PACKET_FMT_12 || headerType == PACKET_FMT_8 {

		bodySizeAry := []byte{0, b[4], b[5], b[6]}
		//fmt.Printf("bodysizeary=%v", bodySizeAry)
		r.bodySize = r.bytes2Int(bodySizeAry)
		r.packetType = int(b[7])

	} else if headerType == PACKET_FMT_4 {

		r.bodySize = preBodySize
		r.packetType = prePacketType

	} else if headerType == PACKET_FMT_1 {

		r.bodySize = preBodySize
		r.packetType = prePacketType
		r.timeStamp = preTimeStamp
	}

	//fmt.Printf("bodysize=%d packetType=%d timestamp=%d\n", r.bodySize, r.packetType, r.timeStamp)

	modChunkCount := r.bodySize % chunkSize
	chunkCount := 0

	if modChunkCount == 0 {
		chunkCount = r.bodySize/chunkSize - 1
		modChunkCount = chunkSize
	} else {

		chunkCount = (r.bodySize - modChunkCount) / chunkSize
	}

	r.packetLen = headerLen + r.bodySize + chunkCount

	//fmt.Println("packetlen=", r.packetLen, "mod count=", modChunkCount)

	if r.packetLen > dataLen {
		fmt.Println("packet len > dataLen error")
		return false
	}

	var bufferData bytes.Buffer

	if chunkCount > 0 {
		//r.bodyData = b[headerLen : headerLen+chunkSize]
		bufferData.Write(b[headerLen : headerLen+chunkSize])

		var i int
		var offset int
		for i = 1; i < chunkCount; i++ {

			offset = (headerLen + chunkSize*i) + i
			bufferData.Write(b[offset : offset+chunkSize])
			//r.bodyData = append(r.bodyData, b[offset:offset+chunkSize])
			//fmt.Printf("offset=%d i=%d headlen=%d\n", offset, i, headerLen)
		}

		if modChunkCount > 0 {
			//fmt.Printf("offset=%d i=%d headlen=%d\n", offset, i, headerLen)
			offset := (headerLen + chunkSize*i) + i
			bufferData.Write(b[offset : offset+modChunkCount])
			//r.bodyData = append(r.bodyData, b[offset:offset+chunkSize])
		}

	} else {
		bufferData.Write(b[headerLen:r.packetLen])
	}

	r.bodyData = bufferData.Bytes()

	//fmt.Println("body len=", len(r.bodyData), "body=", r.bodyData)

	return true
}

func (r *RtmpPacket) GetPacketLen() int {
	return r.packetLen
}

func (r *RtmpPacket) checkEnoughHeader(headerType int, dataLen int) (int, bool) {
	headerLen := 0

	switch {
	case headerType == PACKET_FMT_12:
		headerLen = 12
	case headerType == PACKET_FMT_8:
		headerLen = 8
	case headerType == PACKET_FMT_4:
		headerLen = 4
	case headerType == PACKET_FMT_1:
		headerLen = 1
	}

	if dataLen >= headerLen {
		return headerLen, true
	}

	return headerLen, false
}

func (r *RtmpPacket) bytes2Int(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)
	var outValue int32
	binary.Read(bytesBuffer, binary.BigEndian, &outValue)

	return int(outValue)

}

func (r *RtmpPacket) GetPacketType() int {
	return r.packetType
}

func (r *RtmpPacket) GetBodyData() []byte {
	return r.bodyData
}

func (r *RtmpPacket) AckPacket(chunkStreamId byte, ackSize uint32) []byte {
	var bufData bytes.Buffer
	ackByteAry := make([]byte, 4)

	bufData.WriteByte(chunkStreamId)
	bufData.Write([]byte{0x00, 0x00, 0x00})
	bufData.Write([]byte{0x00, 0x00, 0x04})
	bufData.WriteByte(0x05)
	bufData.Write([]byte{0x00, 0x00, 0x00, 0x00})

	binary.BigEndian.PutUint32(ackByteAry, ackSize)

	bufData.Write(ackByteAry)

	return bufData.Bytes()
}

func (r *RtmpPacket) GetChunkStreamId() byte {
	return r.chunkStreamId
}

func (r *RtmpPacket) SetPeerBandwidthPacket(chunkStreamId byte, bandwidthSize uint32) []byte {
	var bufData bytes.Buffer
	bandwidthAry := make([]byte, 4)

	bufData.WriteByte(chunkStreamId)
	bufData.Write([]byte{0x00, 0x00, 0x00})
	bufData.Write([]byte{0x00, 0x00, 0x05})
	bufData.WriteByte(0x06)
	bufData.Write([]byte{0x00, 0x00, 0x00, 0x00})

	binary.BigEndian.PutUint32(bandwidthAry, bandwidthSize)

	bufData.Write(bandwidthAry)
	bufData.WriteByte(0x02)

	return bufData.Bytes()
}

func (r *RtmpPacket) ControlMessagePacket(chunkStreamId byte, eventType uint16) []byte {
	var bufData bytes.Buffer
	eventTypeAry := make([]byte, 2)

	bufData.WriteByte(chunkStreamId)
	bufData.Write([]byte{0x00, 0x00, 0x00})
	bufData.Write([]byte{0x00, 0x00, 0x06})
	bufData.WriteByte(0x04)
	bufData.Write([]byte{0x00, 0x00, 0x00, 0x00})

	binary.BigEndian.PutUint16(eventTypeAry, eventType)

	bufData.Write(eventTypeAry)
	bufData.Write([]byte{0x00, 0x00, 0x00, 0x00})

	return bufData.Bytes()
}

func (r *RtmpPacket) InvokeMessage(headerType byte, chunkStreamId byte, timeStamp int, bodyData []byte, chunkSize int) []byte {
	var bufData bytes.Buffer
	var tempBodySizeBuf = make([]byte, 4)

	var headerFormat byte
	headerFormat = headerType<<6 | chunkStreamId

	//fmt.Printf("header format=%d\n", headerFormat)

	binary.BigEndian.PutUint32(tempBodySizeBuf, uint32(len(bodyData)))

	bufData.WriteByte(headerFormat)

	switch headerType {
	case PACKET_FMT_12:
		bufData.Write([]byte{0x00, 0x00, 0x00})
		bufData.Write(tempBodySizeBuf[1:])
		bufData.WriteByte(0x14)
		bufData.Write([]byte{0x00, 0x00, 0x00, 0x00})
	case PACKET_FMT_8:
		bufData.Write([]byte{0x00, 0x00, 0x00})
		bufData.Write(tempBodySizeBuf[1:])
		bufData.WriteByte(0x14)
	case PACKET_FMT_4:
		bufData.Write([]byte{0x00, 0x00, 0x00})
	case PACKET_FMT_1:

	}

	if len(bodyData) <= chunkSize {
		bufData.Write(bodyData)
	} else {
		chunkData := r.PackBodyChunk(chunkStreamId, chunkSize, bodyData)
		bufData.Write(chunkData)
	}

	/*
		for i := 12; i < len(bufData.Bytes()); i++ {
			if (i-12)%16 == 0 {
				fmt.Printf("\n")
			}

			fmt.Printf("%02X ", bufData.Bytes()[i])
		}
	*/

	return bufData.Bytes()
}

func (r *RtmpPacket) PackBodyChunk(chunkStreamId byte, chunkSize int, bodyData []byte) []byte {
	dataLen := len(bodyData)
	input := bytes.NewBuffer(bodyData)
	output := new(bytes.Buffer)
	io.CopyN(output, input, int64(chunkSize))
	remain := dataLen - chunkSize
	for {
		output.WriteByte(0xC0 | chunkStreamId)
		if remain > chunkSize {
			io.CopyN(output, input, int64(chunkSize))
			remain -= chunkSize
		} else {
			io.CopyN(output, input, int64(remain))
			break
		}
	}
	return output.Bytes()
}

func (r *RtmpPacket) GetBodySize() int {
	return r.bodySize
}

func (r *RtmpPacket) GetTimeStamp() int {
	return r.timeStamp
}
