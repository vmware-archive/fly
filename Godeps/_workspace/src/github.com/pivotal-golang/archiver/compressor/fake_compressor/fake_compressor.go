package fake_compressor

type FakeCompressor struct {
	Src  string
	Dest string

	CompressError error
}

func (compressor *FakeCompressor) Compress(src, dest string) error {
	if compressor.CompressError != nil {
		return compressor.CompressError
	}

	compressor.Src = src
	compressor.Dest = dest
	return nil
}
