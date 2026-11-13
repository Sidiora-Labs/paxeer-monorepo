// Copyright 2017 The Rust Project Developers.
// Licensed under Apache-2.0 OR MIT.
// Adapted from rust-lang/compiler-builtins src/probestack.rs:
// https://github.com/rust-lang/compiler-builtins/blob/cae3e6ea23739166504f9f9fb50ec070097979d4/src/probestack.rs

// Wasmer's x86_64 stack probe takes the frame size in rax and preserves rax,
// rbp and rsp. Each page is touched before the caller allocates the frame.
core::arch::global_asm!(
    r#"
    .pushsection .text.__rust_probestack,"ax",@progbits
    .globl __rust_probestack
    .hidden __rust_probestack
    .type __rust_probestack,@function
__rust_probestack:
    .cfi_startproc
    pushq %rbp
    .cfi_adjust_cfa_offset 8
    .cfi_offset %rbp, -16
    movq %rsp, %rbp
    .cfi_def_cfa_register %rbp
    movq %rax, %r11
    cmpq $0x1000, %r11
    jna 3f
2:
    subq $0x1000, %rsp
    testq %rsp, 8(%rsp)
    subq $0x1000, %r11
    cmpq $0x1000, %r11
    ja 2b
3:
    subq %r11, %rsp
    testq %rsp, 8(%rsp)
    addq %rax, %rsp
    leave
    .cfi_def_cfa_register %rsp
    .cfi_adjust_cfa_offset -8
    ret
    .cfi_endproc
    .size __rust_probestack, . - __rust_probestack
    .popsection
"#,
    options(att_syntax)
);

#[cfg(test)]
mod tests {
    #[test]
    fn stack_probe_preserves_frame_size_and_stack_pointer() {
        for size in [0usize, 1, 4095, 4096, 4097, 8192, 12289] {
            let frame_size: usize;
            let stack_before: usize;
            let stack_after: usize;
            // SAFETY: These frame sizes fit the test thread's stack. The probe
            // uses its custom ABI, preserves the stack and clobbers only r11
            // and flags, which are declared to the compiler here.
            unsafe {
                core::arch::asm!(
                    "mov {before}, rsp",
                    "call __rust_probestack",
                    "mov {after}, rsp",
                    before = out(reg) stack_before,
                    after = out(reg) stack_after,
                    inlateout("rax") size => frame_size,
                    lateout("r11") _,
                );
            }
            assert_eq!(frame_size, size);
            assert_eq!(stack_after, stack_before);
        }
    }
}
