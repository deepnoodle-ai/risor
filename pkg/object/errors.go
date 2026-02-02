package object

func NewArgsError(fn string, takes, given int) *Error {
	return NewError(newArgsErrorf("args error: %s() takes exactly %d arguments (%d given)",
		fn, takes, given))
}

func NewArgsRangeError(fn string, takesMin, takesMax, given int) *Error {
	if takesMin == takesMax {
		return NewError(newArgsErrorf("args error: %s() takes exactly %d arguments (%d given)",
			fn, takesMin, given))
	}
	if takesMax == takesMin+1 {
		return NewError(newArgsErrorf("args error: %s() takes %d or %d arguments (%d given)",
			fn, takesMin, takesMax, given))
	}
	return NewError(newArgsErrorf("args error: %s() takes between %d and %d arguments (%d given)",
		fn, takesMin, takesMax, given))
}
