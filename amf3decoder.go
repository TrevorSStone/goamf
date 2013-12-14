// Copyright 2013, zhangpeihao All rights reserved.

package amf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

type AMF3Decoder struct {
	stringReferences []string
	objectReferences []interface{}
	classReferences  []ClassDefinition
}

//-----------------------------------------------------------------------
// AMF3 Read functions
func (decoder *AMF3Decoder) AMF3_ReadU29(r Reader) (n uint32, err error) {
	var b byte
	for i := 0; i < 3; i++ {
		b, err = r.ReadByte()
		if err != nil {
			return
		}
		n = (n << 7) + uint32(b&0x7F)
		if (b & 0x80) == 0 {
			return
		}
	}
	b, err = r.ReadByte()
	if err != nil {
		return
	}
	return ((n << 8) + uint32(b)), nil
}

func (decoder *AMF3Decoder) AMF3_ReadUTF8(r Reader) (string, error) {
	var handle uint32
	var err error
	handle, err = decoder.AMF3_ReadU29(r)
	if err != nil {
		return "", err
	}
	reference := handle&uint32(0x01) == uint32(0)
	handle = handle >> 1
	if reference {
		// Todo: reference

		return decoder.stringReferences[handle], nil //errors.New("AMF3 reference unsupported")
	}
	if handle == 0 {
		return "", nil
	}
	data := make([]byte, handle)
	_, err = r.Read(data)
	if err != nil {
		return "", err
	}
	decoder.stringReferences = append(decoder.stringReferences, string(data))
	return string(data), nil
}

func (decoder *AMF3Decoder) AMF3_ReadString(r Reader) (str string, err error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return "", err
	}
	if marker != AMF3_STRING_MARKER {
		return "", errors.New("aType error")
	}
	return decoder.AMF3_ReadUTF8(r)
}

func (decoder *AMF3Decoder) AMF3_ReadInteger(r Reader) (num uint32, err error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return 0, err
	}
	if marker != AMF3_INTEGER_MARKER {
		return 0, errors.New("Type error")
	}
	return decoder.AMF3_ReadU29(r)
}

func (decoder *AMF3Decoder) AMF3_ReadDouble(r Reader) (num float64, err error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return 0, err
	}
	if marker != AMF3_DOUBLE_MARKER {
		return 0, errors.New("Type error")
	}
	err = binary.Read(r, binary.BigEndian, &num)
	return
}

func (decoder *AMF3Decoder) AMF3_ReadObjectName(r Reader) (name string, err error) {
	return decoder.AMF3_ReadUTF8(r)
}

func (decoder *AMF3Decoder) AMF3_ReadObject(r Reader) (obj Object, err error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return nil, err
	}
	if marker != AMF3_OBJECT_MARKER {
		return nil, errors.New("Type error")
	}
	objProp, err := decoder.AMF3_ReadObjectProperty(r)
	return objProp.Object, err
}

func (decoder *AMF3Decoder) AMF3_ReadObjectProperty(r Reader) (TypedObject, error) {
	obj := TypedObject{
		Object: make(Object),
	}
	handle, err := AMF3_ReadU29(r)
	if err != nil {
		return obj, err
	}
	inline := ((handle & 1) != 0)
	handle = handle >> 1

	if inline {
		inlineDefine := ((handle & 1) != 0)
		handle = handle >> 1
		cd := ClassDefinition{}
		if inlineDefine {
			cd.ObjectType, err = decoder.AMF3_ReadObjectName(r)
			if err != nil {
				return obj, err
			}
			cd.Externalizable = ((handle & 1) != 0)
			handle = handle >> 1
			cd.Dynamic = ((handle & 1) != 0)
			handle = handle >> 1
			for i := uint32(0); i < handle; i++ {
				str, err := decoder.AMF3_ReadObjectName(r)
				if err != nil {
					return obj, err
				}
				cd.Members = append(cd.Members, str)
			}
			decoder.classReferences = append(decoder.classReferences, cd)
		} else {
			cd = decoder.classReferences[handle]
		}

		obj.ObjectType = cd.ObjectType
		decoder.objectReferences = append(decoder.objectReferences, obj)

		if cd.Externalizable {
			switch cd.ObjectType {
			case "DSK":
				return decoder.readDSK(r)
			case "DSA":
				return decoder.readDSA(r)
			case "flex.messaging.io.ArrayCollection":
				obj.ObjectType = "flex.messaging.io.ArrayCollection"
				o, err := decoder.AMF3_ReadValue(r)
				obj.Object = Object{
					"array": o,
				}
				return obj, err
			case "com.riotgames.platform.systemstate.ClientSystemStatesNotification", "com.riotgames.platform.broadcast.BroadcastNotification":
				fmt.Println("RIOT OH NO")
				return obj, errors.New("RIOT")
			default:
				fmt.Println("NOT IMPLEMETENED")
				return obj, errors.New("NOT IMPLEMETENED")

			}

		} else {

			for _, key := range cd.Members {
				obj.Object[key], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}

				// name, err := decoder.AMF3_ReadObjectName(r)
				// if err != nil {
				// 	return TypedObject{Object: nil}, err
				// }
				// if name == "" {
				// 	break
				// }
				// if _, ok := obj.Object[name]; ok {
				// 	return TypedObject{Object: nil}, errors.New("object-property exists")
				// }
				// value, err := AMF3_ReadValue(r)
				// if err != nil {
				// 	return TypedObject{Object: nil}, err
				// }
				// obj.Object[name] = value
			}
			if cd.Dynamic {
				fmt.Println("OH NO DYMAIC")
				return obj, errors.New("DYNAMIC")
			}
		}

	} else {
		if int(handle) >= len(decoder.objectReferences) {
			return obj, errors.New("Not enough References")
		}
		objref := decoder.objectReferences[handle]
		if val, ok := objref.(TypedObject); ok {
			return val, err
		}
	}
	return obj, err

}

func (decoder *AMF3Decoder) readDSK(r Reader) (TypedObject, error) {
	obj, err := decoder.readDSA(r)
	if err != nil {
		return obj, err
	}
	obj.ObjectType = "DSK"

	flags, err := readFlags(r)
	if err != nil {
		return obj, err
	}

	for _, flag := range flags {
		decoder.readRemaining(flag, 0, r)
	}
	return obj, nil
}

func (decoder *AMF3Decoder) readDSA(r Reader) (TypedObject, error) {
	obj := TypedObject{
		Object:     make(Object),
		ObjectType: "DSA",
	}
	flags, err := readFlags(r)
	if err != nil {
		return obj, err
	}
	for i, flag := range flags {
		bits := uint(0)
		if i == 0 {
			if flag&0x01 != 0 {
				obj.Object["body"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x02 != 0 {
				obj.Object["clientId"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x04 != 0 {
				obj.Object["destination"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x08 != 0 {
				obj.Object["headers"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x10 != 0 {
				obj.Object["messageId"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x20 != 0 {
				obj.Object["timeStamp"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x40 != 0 {
				obj.Object["timeToLive"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			bits = 7
		} else if i == 1 {
			if flag&0x01 != 0 {
				temp, err := decoder.AMF3_ReadByteArray(r)
				if err != nil {
					return obj, err
				}
				obj.Object["clientIdBytes"] = temp
				obj.Object["clientId"] = byteArrayToID(temp)
			}
			if flag&0x02 != 0 {
				temp, err := decoder.AMF3_ReadByteArray(r)
				if err != nil {
					return obj, err
				}
				obj.Object["messsageIdBytes"] = temp
				obj.Object["messageId"] = byteArrayToID(temp)
			}
			bits = 2
		}
		decoder.readRemaining(flag, bits, r)
	}
	flags, err = readFlags(r)
	if err != nil {
		return obj, err
	}
	for i, flag := range flags {
		bits := uint(0)
		if i == 0 {
			if flag&0x01 != 0 {
				obj.Object["correlationId"], err = decoder.AMF3_ReadValue(r)
				if err != nil {
					return obj, err
				}
			}
			if flag&0x02 != 0 {
				temp, err := decoder.AMF3_ReadByteArray(r)
				if err != nil {
					return obj, err
				}
				obj.Object["correlationIdBytes"] = temp
				obj.Object["correlationId"] = byteArrayToID(temp)
			}
			bits = 2
		}
		decoder.readRemaining(flag, bits, r)
	}

	return obj, nil

}

func (decoder *AMF3Decoder) readRemaining(flag, bits uint, r Reader) {
	if (flag >> bits) != 0 {
		for i := bits; i < 6; i++ {
			if (flag>>i)&1 != 0 {
				decoder.AMF3_ReadValue(r)
			}
		}
	}
}

func byteArrayToID(data []byte) string {
	id := ""
	for i := 0; i < len(data); i++ {
		switch i {
		case 4, 6, 8, 10:
			id = id + "-"
		}
		id = id + fmt.Sprintf("%02x", data[i])
	}
	return id
}

func readFlags(r Reader) ([]uint, error) {
	var flags []uint
	for {
		flag, err := r.ReadByte()
		if err != nil {
			return flags, err
		}
		flags = append(flags, uint(flag))
		if flag&0x80 == 0 {
			break
		}
	}
	return flags, nil
}

func (decoder *AMF3Decoder) AMF3_ReadByteArray(r Reader) ([]byte, error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return nil, err
	}
	if marker != AMF3_BYTEARRAY_MARKER {
		return nil, errors.New("Type error")
	}
	return decoder.AMF3_readByteArray(r)
}

func (decoder *AMF3Decoder) AMF3_readByteArray(r Reader) ([]byte, error) {
	length, err := AMF3_ReadU29(r)
	if err != nil {
		return nil, err
	}
	if length&uint32(0x01) != uint32(0x01) {
		length = (length >> 1)
		buf := decoder.objectReferences[length]
		if buffer, ok := buf.([]byte); ok {
			return buffer, nil
		}
		return nil, errors.New("Byte Array Reference Not Found")
	}
	length = (length >> 1)
	buf := make([]byte, length)
	n, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != int(length) {
		return nil, errors.New("Read buffer size error")
	}
	decoder.objectReferences = append(decoder.objectReferences, buf)
	return buf, nil
}

func (decoder *AMF3Decoder) AMF3_ReadDate(r Reader) (time.Time, error) {
	handle, err := AMF3_ReadU29(r)
	if err != nil {
		return time.Time{}, err
	}
	inline := handle&1 != 0
	handle = handle >> 1
	if inline {
		var ms float64
		err := binary.Read(r, binary.BigEndian, &ms)
		if err != nil {
			return time.Time{}, err
		}
		t := time.Unix(0, int64(ms*1000000))
		decoder.objectReferences = append(decoder.objectReferences, t)
		return t, nil
	} else {
		obj := decoder.objectReferences[handle]
		if t, ok := obj.(time.Time); ok {
			return t, nil
		}
		return time.Time{}, errors.New("Time Reference Not Found")
	}
}

func (decoder *AMF3Decoder) AMF3_ReadArray(r Reader) (objarr []interface{}, err error) {

	var handle uint32
	handle, err = decoder.AMF3_ReadU29(r)
	if err != nil {
		return objarr, err
	}
	reference := handle&uint32(0x01) == uint32(0)
	handle = handle >> 1
	if reference {

		if obj, ok := decoder.objectReferences[handle].([]interface{}); ok {
			return obj, nil
		} else {
			fmt.Println("Could Not Read Array Reference")
			return obj, errors.New("Could Not Read Array Reference")
		}

	}
	key, err := decoder.AMF3_ReadObjectName(r)
	if err != nil {
		return objarr, err
	}
	if key != "" {
		return objarr, errors.New("Associative arrays are not supported")
	}
	objarr = make([]interface{}, int(handle))
	decoder.objectReferences = append(decoder.objectReferences, objarr)

	for i := 0; i < int(handle); i++ {

		objarr[i], err = decoder.AMF3_ReadValue(r)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	return objarr, nil
}

func (decoder *AMF3Decoder) AMF3_ReadValue(r Reader) (value interface{}, err error) {
	marker, err := ReadMarker(r)

	if err != nil {
		return 0, err
	}
	switch marker {
	case AMF3_UNDEFINED_MARKER:
		return Undefined{}, nil
	case AMF3_NULL_MARKER:
		return nil, nil
	case AMF3_FALSE_MARKER:
		return false, nil
	case AMF3_TRUE_MARKER:
		return true, nil
	case AMF3_INTEGER_MARKER:
		return decoder.AMF3_ReadU29(r)
	case AMF3_DOUBLE_MARKER:
		var num float64
		err = binary.Read(r, binary.BigEndian, &num)
		return num, err
	case AMF3_STRING_MARKER:
		return decoder.AMF3_ReadUTF8(r)
	case AMF3_ARRAY_MARKER:
		return decoder.AMF3_ReadArray(r)
	case AMF3_OBJECT_MARKER:
		return decoder.AMF3_ReadObjectProperty(r)
	case AMF3_BYTEARRAY_MARKER:
		return decoder.AMF3_readByteArray(r)
	case AMF3_DATE_MARKER:
		return decoder.AMF3_ReadDate(r)
	}

	return nil, errors.New(fmt.Sprintf("Unknown marker type: %d", marker))
}

func (decoder *AMF3Decoder) ReadValue(r Reader) (value interface{}, err error) {
	marker, err := ReadMarker(r)
	if err != nil {
		return 0, err
	}
	switch marker {
	case AMF0_NUMBER_MARKER:
		var num float64
		err = binary.Read(r, binary.BigEndian, &num)
		return num, err
	case AMF0_BOOLEAN_MARKER:
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return bool(b != 0), nil
	case AMF0_STRING_MARKER:
		return ReadUTF8(r)
	case AMF0_OBJECT_MARKER:
		return ReadObjectProperty(r)
	case AMF0_MOVIECLIP_MARKER:
		return nil, errors.New("Unsupported type: movie clip")
	case AMF0_NULL_MARKER:
		return nil, nil
	case AMF0_UNDEFINED_MARKER:
		return Undefined{}, nil
	case AMF0_REFERENCE_MARKER:
		return nil, errors.New("Unsupported type: reference")
	case AMF0_ECMA_ARRAY_MARKER:
		// Decode ECMA Array to object
		arrLen := make([]byte, 4)
		_, err = r.Read(arrLen)
		if err != nil {
			return nil, err
		}
		obj, err := ReadObjectProperty(r)
		if err != nil {
			return nil, err
		}
		return obj, nil
	case AMF0_OBJECT_END_MARKER:
		return nil, errors.New("Marker error, Object end")
	case AMF0_STRICT_ARRAY_MARKER:
		return nil, errors.New("Unsupported type: strict array")
	case AMF0_DATE_MARKER:
		return nil, errors.New("Unsupported type: date")
	case AMF0_LONG_STRING_MARKER:
		return ReadUTF8Long(r)
	case AMF0_UNSUPPORTED_MARKER:
		return nil, errors.New("Unsupported type: unsupported")
	case AMF0_RECORDSET_MARKER:
		return nil, errors.New("Unsupported type: recordset")
	case AMF0_XML_DOCUMENT_MARKER:
		return nil, errors.New("Unsupported type: XML document")
	case AMF0_TYPED_OBJECT_MARKER:
		return nil, errors.New("Unsupported type: typed object")
	case AMF0_ACMPLUS_OBJECT_MARKER:
		return decoder.AMF3_ReadValue(r)
	}
	return nil, errors.New(fmt.Sprintf("Unknown marker type: %d", marker))
}
