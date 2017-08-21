package rtmp

import "bytes"
import "encoding/binary"
import "math"
import "fmt"

const (
	AMF0_NUMBER       = 0X00
	AMF0_BOOLEAN      = 0X01
	AMF0_STRING       = 0X02
	AMF0_OBJECT       = 0X03
	AMF0_MOVIECLIP    = 0X04
	AMF0_NULL         = 0X05
	AMF0_UNDEFINED    = 0X06
	AMF0_REFERENCE    = 0X07
	AMF0_ECMA_ARRAY   = 0X08
	AMF0_OBJECT_END   = 0X09
	AMF0_STRICT_ARRAY = 0X0A
	AMF0_DATE         = 0X0B
	AMF0_LONG_STRING  = 0X0C
	AMF0_UNSUPPORTED  = 0X0D
	AMF0_RECORDSET    = 0X0E
	AMF0_XML_DOCUMENT = 0X0F
	AMF0_TYPED_OBJECT = 0X10
)

type Amf0 struct {
	offset  int
	buf     bytes.Buffer
	aryData []byte
}

const (
	AMF_DATA_ARRAY  = 0XF0
	AMF_DATA_OBJECT = 0XF1
)

type AmfData struct {
	DataType  byte
	NumberVal float64
	StrVal    string
	BoolVal   byte
	ObjMap    map[string]AmfData
	ObjList   []AmfData
}

func (a *Amf0) ReadData(aryData []byte, isProperty bool) AmfData {

	var amfDataObj AmfData

	if !isProperty {
		a.offset = 0
		a.aryData = aryData

	} else {

	}

	if a.offset == 0 {
		amfDataObj.DataType = AMF_DATA_ARRAY
	}

	for {

		if a.offset >= len(aryData) {
			break
		}

		dataType := aryData[a.offset]

		if dataType > 0x10 {
			fmt.Printf("Data type error! this=%p dataType=%d isproperty=%t offset=%d ary=%v", a, dataType, isProperty, a.offset, aryData)
			break
		}

		var amfDataObjRet AmfData

		switch dataType {
		case AMF0_NUMBER:
			amfDataObjRet = a.readNumber()
		case AMF0_BOOLEAN:
			amfDataObjRet = a.readBoolean()
		case AMF0_STRING:
			var ret bool
			amfDataObjRet, ret = a.readString()
			if !ret {
				break
			}
		case AMF0_OBJECT:
			amfDataObjRet = a.readObjectBegin()
		case AMF0_MOVIECLIP:
			//amf0-file-format-specification
			//The Movieclip and Recordset types are not supported for serialization;
			//their markers are retained with a reserved status for future use
			break
		case AMF0_NULL:
			amfDataObjRet = a.readNull()
		case AMF0_UNDEFINED:
			amfDataObjRet = a.readUndefined()
		case AMF0_REFERENCE:
			amfDataObjRet = a.readReference()
		case AMF0_ECMA_ARRAY:
			amfDataObjRet = a.readEcmaArray()
		case AMF0_OBJECT_END:
			a.readObjectEnd()
			break
		case AMF0_STRICT_ARRAY:
			break
		case AMF0_DATE:
			break
		case AMF0_LONG_STRING:
			break
		case AMF0_UNSUPPORTED:
			break
		case AMF0_RECORDSET:
			break
		case AMF0_XML_DOCUMENT:
			break
		case AMF0_TYPED_OBJECT:
			break
		default:
			break

		}

		if isProperty {
			return amfDataObjRet
		}

		amfDataObj.ObjList = append(amfDataObj.ObjList, amfDataObjRet)
	}

	return amfDataObj
}

func (a *Amf0) readNumber() AmfData {
	tempBuf := bytes.NewBuffer(a.aryData[a.offset+1 : a.offset+9])
	var retVal float64

	binary.Read(tempBuf, binary.BigEndian, &retVal)

	a.offset += 9

	var amfData AmfData
	amfData.DataType = AMF0_NUMBER
	amfData.NumberVal = retVal

	return amfData
}

func (a *Amf0) readBoolean() AmfData {
	var boolVal byte = a.aryData[a.offset+1]

	a.offset += 2

	var amfDataRet AmfData
	amfDataRet.DataType = AMF0_BOOLEAN
	amfDataRet.BoolVal = boolVal

	return amfDataRet
}

func (a *Amf0) readString() (AmfData, bool) {

	var amfDataRet AmfData

	tempBuf := bytes.NewBuffer(a.aryData[a.offset+1 : a.offset+3])
	var strLen int16

	binary.Read(tempBuf, binary.BigEndian, &strLen)

	if int(strLen) > (len(a.aryData) - a.offset) {
		return amfDataRet, false
	}

	strAry := a.aryData[a.offset+3 : a.offset+3+int(strLen)]

	a.offset = a.offset + 3 + int(strLen)

	amfDataRet.DataType = AMF0_STRING
	amfDataRet.StrVal = string(strAry)

	return amfDataRet, true
}

func (a *Amf0) readObjectBegin() AmfData {
	a.offset += 1

	var amfDataRet AmfData
	amfDataRet.DataType = AMF_DATA_OBJECT

	amfDataRet.ObjMap = a.readObject()

	return amfDataRet
}

func (a *Amf0) readObjectEnd() {
	a.offset += 1
}

func (a *Amf0) readNull() AmfData {
	a.offset += 1

	var amfData AmfData
	amfData.DataType = AMF0_NULL

	return amfData
}

func (a *Amf0) readUndefined() AmfData {
	a.offset += 1

	var amfData AmfData
	amfData.DataType = AMF0_UNDEFINED

	return amfData

}

func (a *Amf0) readReference() AmfData {
	tempBuf := bytes.NewBuffer(a.aryData[a.offset+1 : a.offset+3])
	var refLen int16

	binary.Read(tempBuf, binary.BigEndian, &refLen)

	a.offset += 3

	var amfData AmfData
	amfData.DataType = AMF0_REFERENCE

	return amfData
}

func (a *Amf0) readEcmaArray() AmfData {

	tempBuf := bytes.NewBuffer(a.aryData[a.offset+1 : a.offset+5])
	var aryLen uint32

	binary.Read(tempBuf, binary.BigEndian, &aryLen)

	a.offset += 5

	var amfDataRet AmfData
	amfDataRet.DataType = AMF_DATA_OBJECT

	/*
		for i := 0; i < int(aryLen); i++ {
			key, amfData := a.readObject()
			amfData.ObjMap[key] = amfData
		}
	*/

	amfDataRet.ObjMap = a.readObject()

	return amfDataRet
}

func (a *Amf0) readObject() map[string]AmfData {

	var mapAmfDataTemp = make(map[string]AmfData)

	for {
		len, key := a.readPropertyKey()
		if len == 0 {
			a.offset += 1
			return mapAmfDataTemp
		}

		amfData := a.ReadData(a.aryData, true)

		mapAmfDataTemp[key] = amfData

	}

	return mapAmfDataTemp
}

func (a *Amf0) readPropertyKey() (uint16, string) {

	tempBuf := bytes.NewBuffer(a.aryData[a.offset : a.offset+2])
	var keyLen uint16

	binary.Read(tempBuf, binary.BigEndian, &keyLen)

	key := a.aryData[a.offset+2 : a.offset+2+int(keyLen)]

	a.offset = a.offset + 2 + int(keyLen)

	return keyLen, string(key)

}

func (a *Amf0) InitWrite() {
	a.offset = 0
}

func (a *Amf0) WriteNull() {
	a.buf.WriteByte(0x05)
}

func (a *Amf0) WriteNumber(number float64) {
	tempAry := make([]byte, 8)
	bits := math.Float64bits(number)
	binary.BigEndian.PutUint64(tempAry, bits)

	a.buf.WriteByte(0x00)
	a.buf.Write(tempAry)
}

func (a *Amf0) WriteString(strVal string) {
	strLen := len(strVal)
	tempAry := make([]byte, 2)

	binary.BigEndian.PutUint16(tempAry, uint16(strLen))

	a.buf.WriteByte(0x02)
	a.buf.Write(tempAry)
	a.buf.Write([]byte(strVal))

}

func (a *Amf0) WriteObjectBegin() {
	a.buf.WriteByte(0x03)
}

func (a *Amf0) WriteObjectEnd() {
	tempAry := []byte{0x00, 0x00, 0x09}
	a.buf.Write(tempAry)
}

func (a *Amf0) WritePropertyKey(strVal string) {
	strLen := len(strVal)
	tempAry := make([]byte, 2)

	binary.BigEndian.PutUint16(tempAry, uint16(strLen))

	a.buf.Write(tempAry)
	a.buf.Write([]byte(strVal))

}

func (a *Amf0) WritePropertyString(strKey string, strVal string) {
	a.WritePropertyKey(strKey)
	a.WriteString(strVal)
}

func (a *Amf0) WritePropertyNumber(strKey string, number float64) {
	a.WritePropertyKey(strKey)
	a.WriteNumber(number)
}

func (a *Amf0) WriteEcmaAryBegin(aryCount uint32) {
	tempAry := make([]byte, 4)
	binary.BigEndian.PutUint32(tempAry, aryCount)

	a.buf.WriteByte(0x08)
	a.buf.Write(tempAry)
}

func (a *Amf0) GetData() []byte {
	return a.buf.Bytes()
}

func (a *Amf0) GetCommand(aryData []byte) string {

	if aryData[0] != 0x02 {
		return ""
	}

	tempBuf := bytes.NewBuffer(aryData[1:3])
	var strLen int16

	binary.Read(tempBuf, binary.BigEndian, &strLen)

	if int(strLen) > (len(aryData) - 3) {
		return ""
	}

	strAry := aryData[3 : 3+int(strLen)]

	return string(strAry)

}
