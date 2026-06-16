package cli

// fionread is the FIONREAD ioctl request returning the bytes immediately
// readable on a fd. x/sys/unix does not export a named const for it on every
// GOOS, and the value is ABI-stable per platform. On Linux FIONREAD == TIOCINQ.
const fionread = 0x541b // FIONREAD
