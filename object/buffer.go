package object

import (
	"bytes"
	"context"
	"fmt"

	"github.com/risor-io/risor/op"
)

type Buffer struct {
	value *bytes.Buffer
}

func (b *Buffer) Inspect() string {
	return b.String()
}

func (b *Buffer) Type() Type {
	return BUFFER
}

func (b *Buffer) Value() *bytes.Buffer {
	return b.value
}

func (b *Buffer) Interface() interface{} {
	return b.value
}

func (b *Buffer) String() string {
	return fmt.Sprintf("buffer(%q)", b.value.String())
}

func (b *Buffer) Compare(other Object) (int, error) {
	switch other := other.(type) {
	case *Buffer:
		return bytes.Compare(b.value.Bytes(), other.value.Bytes()), nil
	case *String:
		return bytes.Compare(b.value.Bytes(), []byte(other.Value())), nil
	case *ByteSlice:
		return bytes.Compare(b.value.Bytes(), other.Value()), nil
	default:
		return 0, TypeErrorf("type error: unable to compare buffer and %s", other.Type())
	}
}

func (b *Buffer) Equals(other Object) Object {
	if b == other {
		return True
	}
	return False
}

func (b *Buffer) IsTruthy() bool {
	return b.value.Len() > 0
}

func (b *Buffer) SetAttr(name string, value Object) error {
	return TypeErrorf("type error: buffer object has no attribute %q", name)
}

func (b *Buffer) GetAttr(name string) (Object, bool) {
	switch name {
	case "bytes":
		return NewBuiltin("bytes", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.bytes", 0, numArgs)
			}
			return NewByteSlice(b.value.Bytes())
		}), true
	case "len":
		return NewBuiltin("len", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.len", 0, numArgs)
			}
			return NewInt(int64(b.value.Len()))
		}), true
	case "cap":
		return NewBuiltin("cap", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.cap", 0, numArgs)
			}
			return NewInt(int64(b.value.Cap()))
		}), true
	case "available":
		return NewBuiltin("available", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.available", 0, numArgs)
			}
			return NewInt(int64(b.value.Available()))
		}), true
	case "truncate":
		return NewBuiltin("truncate", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 1 {
				return NewArgsError("buffer.truncate", 1, numArgs)
			}
			size, err := AsInt(args[0])
			if err != nil {
				return err
			}
			if size < 0 || size > int64(b.value.Len()) {
				return Errorf("value error: buffer.truncate: size %d out of range", size)
			}
			b.value.Truncate(int(size))
			return Nil
		}), true
	case "read":
		return NewBuiltin("read", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs > 1 {
				return NewArgsRangeError("buffer.read", 0, 1, numArgs)
			}
			amount := int64(b.value.Len())
			if numArgs == 1 {
				var err *Error
				amount, err = AsInt(args[0])
				if err != nil {
					return err
				}
			}
			p := make([]byte, amount)
			n, err := b.Read(p)
			if err != nil {
				if err.Error() == "EOF" {
					return NewByteSlice([]byte{})
				}
				return NewError(err)
			}
			return NewByteSlice(p[:n])
		}), true
	case "reset":
		return NewBuiltin("reset", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.reset", 0, numArgs)
			}
			b.value.Reset()
			return Nil
		}), true
	case "write":
		return NewBuiltin("write", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 1 {
				return NewArgsError("buffer.write", 1, numArgs)
			}
			bytes, err := AsBytes(args[0])
			if err != nil {
				return err
			}
			n, writeErr := b.Write(bytes)
			if writeErr != nil {
				return NewError(writeErr)
			}
			return NewInt(int64(n))
		}), true
	case "string":
		return NewBuiltin("string", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs != 0 {
				return NewArgsError("buffer.string", 0, numArgs)
			}
			return NewString(b.value.String())
		}), true
	case "read_string":
		return NewBuiltin("read_string", func(ctx context.Context, args ...Object) Object {
			numArgs := len(args)
			if numArgs > 1 {
				return NewArgsRangeError("buffer.read_string", 0, 1, numArgs)
			}
			delim, err := AsByte(args[0])
			if err != nil {
				return err
			}
			s, readErr := b.value.ReadString(delim)
			if readErr != nil {
				return NewError(readErr)
			}
			return NewString(s)
		}), true
	}
	return nil, false
}

func (b *Buffer) RunOperation(opType op.BinaryOpType, right Object) Object {
	switch right := right.(type) {
	case *Buffer:
		return b.runOperationBytes(opType, right)
	case *String:
		return b.runOperationString(opType, right)
	case *ByteSlice:
		return b.runOperationByteSlice(opType, right)
	default:
		return TypeErrorf("type error: unsupported operation for buffer: %v on type %s", opType, right.Type())
	}
}

func (b *Buffer) runOperationBytes(opType op.BinaryOpType, right *Buffer) Object {
	switch opType {
	case op.Add:
		if _, err := b.value.Write(right.value.Bytes()); err != nil {
			return NewError(err)
		}
		return Nil
	default:
		return TypeErrorf("type error: unsupported operation for buffer: %v on type %s", opType, right.Type())
	}
}

func (b *Buffer) runOperationByteSlice(opType op.BinaryOpType, right *ByteSlice) Object {
	switch opType {
	case op.Add:
		if _, err := b.value.Write(right.value); err != nil {
			return NewError(err)
		}
		return Nil
	default:
		return TypeErrorf("type error: unsupported operation for buffer: %v on type %s", opType, right.Type())
	}
}

func (b *Buffer) runOperationString(opType op.BinaryOpType, right *String) Object {
	switch opType {
	case op.Add:
		if _, err := b.value.WriteString(right.value); err != nil {
			return NewError(err)
		}
		return Nil
	default:
		return TypeErrorf("type error: unsupported operation for buffer: %v on type %s", opType, right.Type())
	}
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	return b.value.Read(p)
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	return b.value.Write(p)
}

func (b *Buffer) Cost() int {
	return len(b.value.Bytes())
}

func (b *Buffer) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", b.value.String())), nil
}

func NewBuffer(buf *bytes.Buffer) *Buffer {
	if buf == nil {
		buf = bytes.NewBuffer(nil)
	}
	return &Buffer{value: buf}
}

func NewBufferFromBytes(value []byte) *Buffer {
	return &Buffer{value: bytes.NewBuffer(value)}
}
