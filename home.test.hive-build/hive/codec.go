package hive

type CodecCodec[E any] struct {
	Secret Secret
}

// String keeps a secret out of anything that prints a codec.
func (c CodecCodec[E]) String() string { return "codec" }

// CodecWith evaluates a held codec where it is used, and answers with value.
func CodecWith[T any, E any](value T, _ CodecCodec[E]) T { return value }
