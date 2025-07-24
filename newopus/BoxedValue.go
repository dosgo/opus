package opus

type BoxedValueByte struct {
	Val int8
}

func BoxedValueByte(v int8) BoxedValueByte {
	return BoxedValueByte{Val: v}
}

type BoxedValueShort struct {
	Val int16
}

func BoxedValueShort(v int16) BoxedValueShort {
	return BoxedValueShort{Val: v}
}

type BoxedValueInt struct {
	Val int32
}

func BoxedValueInt(v int32) BoxedValueInt {
	return BoxedValueInt{Val: v}
}
