package rtmp

import "time"
import "math/rand"
import "encoding/binary"

type HandShakeBuf struct {
	aryData []byte
}

func (h *HandShakeBuf) InitBuf() {

	h.aryData = make([]byte, 3073)

	h.aryData[0] = 0x03
	curTs := time.Now()

	intBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(intBuf, uint32(curTs.Unix()))

	for i := 0; i < 4; i++ {
		h.aryData[i+1] = intBuf[i]
	}

	for i := 0; i < 4; i++ {
		h.aryData[i+5] = 0x0
	}

	for i := 9; i < 3073; i++ {
		h.aryData[i] = byte(rand.Intn(256))
	}

}

func (h *HandShakeBuf) GetBuf() []byte {
	return h.aryData
}
