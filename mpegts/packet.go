package mpegts



// Packet represents a packet
// https://en.wikipedia.org/wiki/MPEG_transport_stream
type Packet struct {
	Bytes           []byte // This is the whole packet content
	Header          *MpegTsHeader
	AdaptationField *MpegTsHeaderAdaptationField
	Payload         []byte // This is only the payload content
}




// ios13818-1-CN.pdf 77
//
// Descriptor
//

type MpegTsDescriptor struct {
	Tag    byte // 8 bits 标识每一个描述符
	Length byte // 8 bits 指定紧随 descriptor_length 字段的描述符的字节数
	Data   []byte
}

// 前面32bit的数据即TS分组首部,它指出了这个分组的属性
type MpegTsHeader struct {
	SyncByte                   byte   // 8 bits  同步字节,固定为0x47,表示后面是一个TS分组，当然，后面包中的数据是不会出现0x47的
	TransportErrorIndicator    byte   // 1 bit  传输错误标志位,一般传输错误的话就不会处理这个包了
	PayloadUnitStartIndicator  byte   // 1 bit  负载单元开始标志(packet不满188字节时需填充).为1时,表示在4个字节后,有一个调整字节,根据后面有效负载的内容不同功能也不同
	TransportPriority          byte   // 1 bit  传输优先级,1表示高优先级
	Pid                        uint16 // 13 bits Packet ID号码,唯一的号码对应不同的包.为0表示携带的是PAT表,有效负载数据的类型
	TransportScramblingControl byte   // 2 bits  加密标志位(00:未加密;其他表示已加密)

	// 2 bits  附加区域控制.表示TS分组首部后面是否跟随有调整字段和有效负载.
	// 01仅含有效负载(没有adaptation_field),
	// 10仅含调整字段(没有Payload),
	// 11含有调整字段和有效负载(有adaptation_field,adaptation_field之后是Payload).
	// 为00的话解码器不进行处理.空分组没有调整字段
	AdaptionFieldControl       byte
	ContinuityCounter          byte   // 4 bits  包递增计数器.范围0-15,具有相同的PID的TS分组传输时每次加1,到15后清0.不过,有些情况下是不计数的.

}




// parsePacketHeader parses the packet header
func parsePacketHeader(input []byte) *MpegTsHeader {

	header := &MpegTsHeader{}

	header.SyncByte =input[0]
	header.TransportErrorIndicator = input[1] >> 7

	// | 0000 0000 | 0100 0000 | 0000 0000 | 0000 0000 |
	header.PayloadUnitStartIndicator = (input[1] >> 6) & 0x01
	// | 0000 0000 | 0010 0000 | 0000 0000 | 0000 0000 |
	header.TransportPriority = (input[1] >> 5) & 0x01

	// | 0000 0000 | 0001 1111 | 1111 1111 | 0000 0000 |
	header.Pid = uint16((input[1] &0x1F) << 8  | input[2])

	// | 0000 0000 | 0000 0000 | 0000 0000 | 1100 0000 |
	header.TransportScramblingControl =  input[3] >> 6;

	// | 0000 0000 | 0000 0000 | 0000 0000 | 0011 0000 |
	// 0x30 , 0x20 -> adaptation_field, 0x10 -> Payload
	header.AdaptionFieldControl =  input[3] >> 4 & 0x03;

	// | 0000 0000 | 0000 0000 | 0000 0000 | 0000 1111 |
	header.ContinuityCounter =  input[3] & 0x0F;


	return  header
}

// 调整字段,只可能出现在每一帧的开头(当含有pcr的时候),或者结尾(当不满足188个字节的时候)
// adaptionFieldControl 00 -> 高字节代表调整字段, 低字节代表负载字段 0x20 0x10
// PCR字段编码在MPEG-2 TS包的自适应字段(Adaptation field)的6个Byte中,其中6 bits为预留位,42 bits为有效位()
// MpegTsHeaderAdaptationField + stuffing bytes
type MpegTsHeaderAdaptationField struct {
	// 8bits 本区域除了本字节剩下的长度(不包含本字节!!!切记), if adaptationFieldLength > 0, 那么就有下面8个字段.
	// adaptation_field_length 值必须在 0 到 182 的区间内.当 adaptation_field_control 值为'10'时,
	// adaptation_field_length 值必须为 183
	AdaptationFieldLength             byte


	// 1bit 置于"1"时,指示当前传输流包的不连续性状态为真
	// .当 discontinuity_indicator 设置为"0"或不存在时,不连续性状态为假.
	// 不连续性指示符用于指示两种类型的不连续性,系统时间基不连续性和 continuity_counter 不连续性.
	DiscontinuityIndicator            byte

	// 1bit 指示当前的传输流包以及可能的具有相同 PID 的后续传输流包,
	// 在此点包含有助于随机接入的某些信息.特别的,该比特置于"1"时,
	// 在具有当前 PID 的传输流包的有效载荷中起始的下一个 PES 包必须包含一个 discontinuity_indicator 字段中规定的基本流接入点.
	// 此外,在视频情况中,显示时间标记必须在跟随基本流接入点的第一图像中存在
	RandomAccessIndicator             byte

	// 1bit 在具有相同 PID 的包之间,它指示此传输流包有效载荷内承载的基本流数据的优先级.1->指示该有效载荷具有比其他传输流包有效载荷更高的优先级
	ElementaryStreamPriorityIndicator byte

	// 1bit 1->指示 adaptation_field 包含以两部分编码的 PCR 字段.0->指示自适应字段不包含任何 PCR 字段
	PCRFlag                           byte

	// 1bit 1->指示 adaptation_field 包含以两部分编码的 OPCR字段.0->指示自适应字段不包含任何 OPCR 字段
	OPCRFlag                          byte

	// 1bit 1->指示 splice_countdown 字段必须在相关自适应字段中存在,指定拼接点的出现.0->指示自适应字段中 splice_countdown 字段不存在
	SplicingPointFlag                 byte

	// 1bit 1->指示自适应字段包含一个或多个 private_data 字节.0->指示自适应字段不包含任何 private_data 字节
	TrasportPrivateDataFlag           byte
	AdaptationFieldExtensionFlag      byte // 1bit 1->指示自适应字段扩展的存在.0->指示自适应字段中自适应字段扩展不存在

	// Optional Fields
	ProgramClockReferenceBase              uint64 // 33 bits pcr
	Reserved1                              byte   // 6 bits
	ProgramClockReferenceExtension         uint16 // 9 bits
	OriginalProgramClockReferenceBase      uint64 // 33 bits opcr
	Reserved2                              byte   // 6 bits
	OriginalProgramClockReferenceExtension uint16 // 9 bits
	SpliceCountdown                        byte   // 8 bits
	TransportPrivateDataLength             byte   // 8 bits 指定紧随传输private_data_length 字段的 private_data 字节数. private_data 字节数不能使专用数据扩展超出自适应字段的范围
	PrivateDataByte                        byte   // 8 bits 不通过 ITU-T|ISO/IEC 指定
	AdaptationFieldExtensionLength         byte   // 8 bits 指定紧随此字段的扩展的自适应字段数据的字节数,包括要保留的字节(如果存在)
	LtwFlag                                byte   // 1 bit 1->指示 ltw_offset 字段存在
	PiecewiseRateFlag                      byte   // 1 bit 1->指示 piecewise_rate 字段存在
	SeamlessSpliceFlag                     byte   // 1 bit 1->指示 splice_type 以及 DTS_next_AU 字段存在. 0->指示无论是 splice_type 字段还是 DTS_next_AU 字段均不存在

	// Optional Fields
	LtwValidFlag  byte   // 1 bit 1->指示 ltw_offset 的值必将生效.0->指示 ltw_offset 字段中该值未定义
	LtwOffset     uint16 // 15 bits 其值仅当 ltw_valid 标志字段具有'1'值时才定义.定义时,法定时间窗补偿以(300/fs)秒为度量单位,其中 fs 为此 PID 所归属的节目的系统时钟频率
	Reserved3     byte   // 2 bits 保留
	PiecewiseRate uint32 // 22 bits 只要当 ltw_flag 和 ltw_valid_flag 均置于‘1’时,此 22 比特字段的含义才确定
	SpliceType    byte   // 4 bits
	DtsNextAU     uint64 // 33 bits (解码时间标记下一个存取单元)

	// stuffing bytes
	// 此为固定的 8 比特值等于'1111 1111',能够通过编码器插入.它亦能被解码器丢弃
}

// parsePacket parses a packet
func parsePacket(input []byte) (p *Packet, err error) {
	// Packet must start with a sync byte
	if input[0] != syncByte {
		err = ErrPacketMustStartWithASyncByte
		return
	}

	// Init
	p = &Packet{Bytes: input}

	// In case packet size is bigger than 188 bytes, we don't care for the first bytes
	//input =input[len(input)-188+1:]

	// Parse header
	//调控字节
	// | 0010 0000 |
	// adaptionFieldControl
	// 表示TS分组首部后面是否跟随有调整字段和有效负载.
	// 01仅含有效负载(没有adaptation_field)
	// 10仅含调整字段(没有Payload)
	// 11含有调整字段和有效负载(有adaptation_field,adaptation_field之后是Payload).
	// 为00的话解码器不进行处理.空分组没有调整字段
	// 当值为'11时,adaptation_field_length 值必须在0 到182 的区间内.
	// 当值为'10'时,adaptation_field_length 值必须为183.
	// 对于承载PES 包的传输流包,只要存在欠充足的PES 包数据就需要通过填充来完全填满传输流包的有效载荷字节.
	// 填充通过规定自适应字段长度比自适应字段中数据元的长度总和还要长来实现,
	// 以致于自适应字段在完全容纳有效的PES 包数据后,有效载荷字节仍有剩余.自适应字段中额外空间采用填充字节填满.

	p.Header = parsePacketHeader(input)

	var HasPayload bool
	var HasAdaptationField bool

	switch p.Header.AdaptionFieldControl {
	case 0:
		//no process
	case 1:
		HasPayload = true
	case 2:
		HasAdaptationField = true
	case 3:
		HasPayload = true
		HasAdaptationField = true
	}


	// Parse adaptation field
	if HasAdaptationField {
		p.AdaptationField = parsePacketAdaptationField(input[4:])
	}

	// Build payload
	if HasPayload {
		p.Payload = i[payloadOffset(p.Header, p.AdaptationField):]
	}
	return
}



// stuffing bytes
// 此为固定的 8 比特值等于'1111 1111',能够通过编码器插入.它亦能被解码器丢弃
// parsePacketAdaptationField parses the packet adaptation field
func parsePacketAdaptationField(i []byte) (a *MpegTsHeaderAdaptationField) {
	// Init
	a = &MpegTsHeaderAdaptationField{}
	var offset int

	// Length
	a.AdaptationFieldLength = i[offset]
	offset += 1

	// Valid length
	if a.AdaptationFieldLength > 0 {
		// Flags
		flags := i[offset]

		a.DiscontinuityIndicator = flags & 0x80
		a.RandomAccessIndicator = flags & 0x40
		a.ElementaryStreamPriorityIndicator = flags & 0x20
		a.PCRFlag = flags & 0x10
		a.OPCRFlag = flags & 0x08
		a.SplicingPointFlag = flags & 0x04
		a.TrasportPrivateDataFlag = flags & 0x02
		a.AdaptationFieldExtensionFlag = flags & 0x01

		offset += 1

		// PCR
		if a.HasPCR {
			a.PCR = parsePCR(i[offset:])
			offset += 6
		}

		// OPCR
		if a.HasOPCR {
			a.OPCR = parsePCR(i[offset:])
			offset += 6
		}

		// Splicing countdown
		if a.HasSplicingCountdown {
			a.SpliceCountdown = int(i[offset])
			offset += 1
		}

		// Transport private data
		if a.HasTransportPrivateData {
			a.TransportPrivateDataLength = int(i[offset])
			offset += 1
			if a.TransportPrivateDataLength > 0 {
				a.TransportPrivateData = i[offset : offset+a.TransportPrivateDataLength]
				offset += a.TransportPrivateDataLength
			}
		}

		// Adaptation extension
		if a.HasAdaptationExtensionField {
			a.AdaptationExtensionField = &PacketAdaptationExtensionField{Length: int(i[offset])}
			offset += 1
			if a.AdaptationExtensionField.Length > 0 {
				// Basic
				a.AdaptationExtensionField.HasLegalTimeWindow = i[offset]&0x80 > 0
				a.AdaptationExtensionField.HasPiecewiseRate = i[offset]&0x40 > 0
				a.AdaptationExtensionField.HasSeamlessSplice = i[offset]&0x20 > 0
				offset += 1

				// Legal time window
				if a.AdaptationExtensionField.HasLegalTimeWindow {
					a.AdaptationExtensionField.LegalTimeWindowIsValid = i[offset]&0x80 > 0
					a.AdaptationExtensionField.LegalTimeWindowOffset = uint16(i[offset]&0x7f)<<8 | uint16(i[offset+1])
					offset += 2
				}

				// Piecewise rate
				if a.AdaptationExtensionField.HasPiecewiseRate {
					a.AdaptationExtensionField.PiecewiseRate = uint32(i[offset]&0x3f)<<16 | uint32(i[offset+1])<<8 | uint32(i[offset+2])
					offset += 3
				}

				// Seamless splice
				if a.AdaptationExtensionField.HasSeamlessSplice {
					a.AdaptationExtensionField.SpliceType = uint8(i[offset]&0xf0) >> 4
					a.AdaptationExtensionField.DTSNextAccessUnit = parsePTSOrDTS(i[offset:])
				}
			}
		}
	}
	return
}


