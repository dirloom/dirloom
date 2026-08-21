package clipboard

// New returns the native clipboard writer for the current operating system.
func New() Writer {
	return newNativeWriter()
}
