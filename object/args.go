package object

func Require(funcName string, count int, args []Object) *Error {
	nArgs := len(args)
	if nArgs != count {
		if count == 1 {
			return NewError(ArgsErrorf(
				"args error: %s() takes exactly 1 argument (%d given)",
				funcName, nArgs))
		}
		return NewError(ArgsErrorf(
			"args error: %s() takes exactly %d arguments (%d given)",
			funcName, count, nArgs))
	}
	return nil
}

func RequireRange(funcName string, min, max int, args []Object) *Error {
	nArgs := len(args)
	if nArgs < min {
		return NewError(ArgsErrorf(
			"args error: %s() takes at least %d %s (%d given)",
			funcName, min, pluralize("argument", nArgs > 1), nArgs))
	} else if nArgs > max {
		return NewError(ArgsErrorf(
			"args error: %s() takes at most %d %s (%d given)",
			funcName, max, pluralize("argument", nArgs > 1), nArgs))
	}
	return nil
}

func pluralize(s string, do bool) string {
	if do {
		return s + "s"
	}
	return s
}
