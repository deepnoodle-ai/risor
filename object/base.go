package object

type base struct{}

func (b *base) GetAttr(name string) (Object, bool) {
	return nil, false
}

func (b *base) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: object has no attribute %q", name)
}

func (b *base) IsTruthy() bool {
	return true
}
