// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/sys"
	"unsafe"
)

// Should be a built-in for unsafe.Pointer?
//go:nosplit
func add(p unsafe.Pointer, x uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

// getg returns the pointer to the current g.
// The compiler rewrites calls to this function into instructions
// that fetch the g directly (from TLS or from the dedicated register).
func getg() *g

// mcall switches from the g to the g0 stack and invokes fn(g),
// where g is the goroutine that made the call.
// mcall saves g's current PC/SP in g->sched so that it can be restored later.
// It is up to fn to arrange for that later execution, typically by recording
// g in a data structure, causing something to call ready(g) later.
// mcall returns to the original goroutine g later, when g has been rescheduled.
// fn must not return at all; typically it ends by calling schedule, to let the m
// run other goroutines.
//
// mcall can only be called from g stacks (not g0, not gsignal).
//
// This must NOT be go:noescape: if fn is a stack-allocated closure,
// fn puts g on a run queue, and g executes before fn returns, the
// closure will be invalidated while it is still executing.
func mcall(fn func(*g))

// systemstack runs fn on a system stack.
//
// It is common to use a func literal as the argument, in order
// to share inputs and outputs with the code around the call
// to system stack:
//
//	... set up y ...
//	systemstack(func() {
//		x = bigcall(y)
//	})
//	... use x ...
//
// For the gc toolchain this permits running a function that requires
// additional stack space in a context where the stack can not be
// split. We don't really need additional stack space in gccgo, since
// stack splitting is handled separately. But to keep things looking
// the same, we do switch to the g0 stack here if necessary.
func systemstack(fn func()) {
	gp := getg()
	mp := gp.m
	if gp == mp.g0 || gp == mp.gsignal {
		fn()
	} else if gp == mp.curg {
		fn1 := func(origg *g) {
			fn()
			gogo(origg)
		}
		mcall(*(*func(*g))(noescape(unsafe.Pointer(&fn1))))
	} else {
		badsystemstack()
	}
}

var badsystemstackMsg = "fatal: systemstack called from unexpected goroutine"

//go:nosplit
//go:nowritebarrierrec
func badsystemstack() {
	sp := stringStructOf(&badsystemstackMsg)
	write(2, sp.str, int32(sp.len))
}

// memclrNoHeapPointers clears n bytes starting at ptr.
//
// Usually you should use typedmemclr. memclrNoHeapPointers should be
// used only when the caller knows that *ptr contains no heap pointers
// because either:
//
// *ptr is initialized memory and its type is pointer-free, or
//
// *ptr is uninitialized memory (e.g., memory that's being reused
// for a new allocation) and hence contains only "junk".
//
// The (CPU-specific) implementations of this function are in memclr_*.s.
//go:noescape
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname reflect_memclrNoHeapPointers reflect.memclrNoHeapPointers
func reflect_memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr) {
	memclrNoHeapPointers(ptr, n)
}

// memmove copies n bytes from "from" to "to".
//go:noescape
//extern __builtin_memmove
func memmove(to, from unsafe.Pointer, n uintptr)

//go:linkname reflect_memmove reflect.memmove
func reflect_memmove(to, from unsafe.Pointer, n uintptr) {
	memmove(to, from, n)
}

//go:noescape
//extern __builtin_memcmp
func memcmp(a, b unsafe.Pointer, size uintptr) int32

// exported value for testing
var hashLoad = float32(loadFactorNum) / float32(loadFactorDen)

//go:nosplit
func fastrand() uint32 {
	mp := getg().m
	// Implement xorshift64+: 2 32-bit xorshift sequences added together.
	// Shift triplet [17,7,16] was calculated as indicated in Marsaglia's
	// Xorshift paper: https://www.jstatsoft.org/article/view/v008i14/xorshift.pdf
	// This generator passes the SmallCrush suite, part of TestU01 framework:
	// http://simul.iro.umontreal.ca/testu01/tu01.html
	s1, s0 := mp.fastrand[0], mp.fastrand[1]
	s1 ^= s1 << 17
	s1 = s1 ^ s0 ^ s1>>7 ^ s0>>16
	mp.fastrand[0], mp.fastrand[1] = s0, s1
	return s0 + s1
}

//go:nosplit
func fastrandn(n uint32) uint32 {
	// This is similar to fastrand() % n, but faster.
	// See https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return uint32(uint64(fastrand()) * uint64(n) >> 32)
}

//go:linkname sync_fastrand sync.fastrand
func sync_fastrand() uint32 { return fastrand() }

// in asm_*.s
//go:noescape
func memequal(a, b unsafe.Pointer, size uintptr) bool

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

//go:noescape
func jmpdefer(fv *funcval, argp uintptr)
func exit1(code int32)
func setg(gg *g)

//extern __builtin_trap
func breakpoint()

func asminit() {}

//go:linkname reflectcall runtime.reflectcall
//go:noescape
func reflectcall(fntype *functype, fn *funcval, isInterface, isMethod bool, params, results *unsafe.Pointer)

func procyield(cycles uint32)

type neverCallThisFunction struct{}

// goexit is the return stub at the top of every goroutine call stack.
// Each goroutine stack is constructed as if goexit called the
// goroutine's entry point function, so that when the entry point
// function returns, it will return to goexit, which will call goexit1
// to perform the actual exit.
//
// This function must never be called directly. Call goexit1 instead.
// gentraceback assumes that goexit terminates the stack. A direct
// call on the stack will cause gentraceback to stop walking the stack
// prematurely and if there is leftover state it may panic.
func goexit(neverCallThisFunction)

// publicationBarrier performs a store/store barrier (a "publication"
// or "export" barrier). Some form of synchronization is required
// between initializing an object and making that object accessible to
// another processor. Without synchronization, the initialization
// writes and the "publication" write may be reordered, allowing the
// other processor to follow the pointer and observe an uninitialized
// object. In general, higher-level synchronization should be used,
// such as locking or an atomic pointer write. publicationBarrier is
// for when those aren't an option, such as in the implementation of
// the memory manager.
//
// There's no corresponding barrier for the read side because the read
// side naturally has a data dependency order. All architectures that
// Go supports or seems likely to ever support automatically enforce
// data dependency ordering.
func publicationBarrier()

// getcallerpc returns the program counter (PC) of its caller's caller.
// getcallersp returns the stack pointer (SP) of its caller's caller.
// The implementation may be a compiler intrinsic; there is not
// necessarily code implementing this on every platform.
//
// For example:
//
//	func f(arg1, arg2, arg3 int) {
//		pc := getcallerpc()
//		sp := getcallersp()
//	}
//
// These two lines find the PC and SP immediately following
// the call to f (where f will return).
//
// The call to getcallerpc and getcallersp must be done in the
// frame being asked about.
//
// The result of getcallersp is correct at the time of the return,
// but it may be invalidated by any subsequent call to a function
// that might relocate the stack in order to grow or shrink it.
// A general rule is that the result of getcallersp should be used
// immediately and can only be passed to nosplit functions.

//go:noescape
func getcallerpc() uintptr

//go:noescape
func getcallersp() uintptr // implemented as an intrinsic on all platforms

// getsp returns the stack pointer (SP) of the caller of getsp.
//go:noinline
func getsp() uintptr { return getcallersp() }

func asmcgocall(fn, arg unsafe.Pointer) int32 {
	throw("asmcgocall")
	return 0
}

// argp used in Defer structs when there is no argp.
const _NoArgs = ^uintptr(0)

//extern __builtin_prefetch
func prefetch(addr unsafe.Pointer, rw int32, locality int32)

func prefetcht0(addr uintptr) {
	prefetch(unsafe.Pointer(addr), 0, 3)
}

func prefetcht1(addr uintptr) {
	prefetch(unsafe.Pointer(addr), 0, 2)
}

func prefetcht2(addr uintptr) {
	prefetch(unsafe.Pointer(addr), 0, 1)
}

func prefetchnta(addr uintptr) {
	prefetch(unsafe.Pointer(addr), 0, 0)
}

// round n up to a multiple of a.  a must be a power of 2.
func round(n, a uintptr) uintptr {
	return (n + a - 1) &^ (a - 1)
}

// checkASM returns whether assembly runtime checks have passed.
func checkASM() bool {
	return true
}

// For gccgo this is in the C code.
func osyield()

//extern __go_syscall6
func syscall(trap uintptr, a1, a2, a3, a4, a5, a6 uintptr) uintptr

// For gccgo, to communicate from the C code to the Go code.
//go:linkname setIsCgo runtime.setIsCgo
func setIsCgo() {
	iscgo = true
}

// For gccgo, to communicate from the C code to the Go code.
//go:linkname setSupportAES runtime.setSupportAES
func setSupportAES(v bool) {
	support_aes = v
}

// Here for gccgo.
func errno() int

// For gccgo these are written in C.
func entersyscall()
func entersyscallblock()

// Get signal trampoline, written in C.
func getSigtramp() uintptr

// The sa_handler field is generally hidden in a union, so use C accessors.
//go:noescape
func getSigactionHandler(*_sigaction) uintptr

//go:noescape
func setSigactionHandler(*_sigaction, uintptr)

// Retrieve fields from the siginfo_t and ucontext_t pointers passed
// to a signal handler using C, as they are often hidden in a union.
// Returns  and, if available, PC where signal occurred.
func getSiginfo(*_siginfo_t, unsafe.Pointer) (sigaddr uintptr, sigpc uintptr)

// Implemented in C for gccgo.
func dumpregs(*_siginfo_t, unsafe.Pointer)

// Implemented in C for gccgo.
func setRandomNumber(uint32)

// Called by gccgo's proc.c.
//go:linkname allocg runtime.allocg
func allocg() *g {
	return new(g)
}

// Throw and rethrow an exception.
func throwException()
func rethrowException()

// Fetch the size and required alignment of the _Unwind_Exception type
// used by the stack unwinder.
func unwindExceptionSize() uintptr

// Called by C code to set the number of CPUs.
//go:linkname setncpu runtime.setncpu
func setncpu(n int32) {
	ncpu = n
}

// Called by C code to set the page size.
//go:linkname setpagesize runtime.setpagesize
func setpagesize(s uintptr) {
	if physPageSize == 0 {
		physPageSize = s
	}
}

const uintptrMask = 1<<(8*sys.PtrSize) - 1

type bitvector struct {
	n        int32 // # of bits
	bytedata *uint8
}

// ptrbit returns the i'th bit in bv.
// ptrbit is less efficient than iterating directly over bitvector bits,
// and should only be used in non-performance-critical code.
// See adjustpointers for an example of a high-efficiency walk of a bitvector.
func (bv *bitvector) ptrbit(i uintptr) uint8 {
	b := *(addb(bv.bytedata, i/8))
	return (b >> (i % 8)) & 1
}

// bool2int returns 0 if x is false or 1 if x is true.
func bool2int(x bool) int {
	if x {
		return 1
	}
	return 0
}

// abort crashes the runtime in situations where even throw might not
// work. In general it should do something a debugger will recognize
// (e.g., an INT3 on x86). A crash in abort is recognized by the
// signal handler, which will attempt to tear down the runtime
// immediately.
func abort()

// usestackmaps is true if stack map (precise stack scan) is enabled.
var usestackmaps bool

// probestackmaps detects whether there are stack maps.
//go:linkname probestackmaps runtime.probestackmaps
func probestackmaps() bool

// For the math/bits packages for gccgo.
//go:linkname getDivideError runtime.getDivideError
func getDivideError() error {
	return divideError
}

// For the math/bits packages for gccgo.
//go:linkname getOverflowError runtime.getOverflowError
func getOverflowError() error {
	return overflowError
}
