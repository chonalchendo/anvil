package cli

// fionread is the FIONREAD ioctl request returning the bytes immediately
// readable on a fd. x/sys/unix does not export a named const for it on every
// GOOS, and the value is ABI-stable per platform. The darwin/BSD value encodes
// _IOR('f', 127, int).
const fionread = 0x4004667f // FIONREAD
