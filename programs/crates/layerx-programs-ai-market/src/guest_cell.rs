#![allow(unsafe_code)]

use core::{
    cell::{Cell, UnsafeCell},
    ops::{Deref, DerefMut},
};

#[cfg(target_feature = "atomics")]
compile_error!("guest cells require the single-threaded Programs runtime");

pub(crate) struct GuestCell<T> {
    value: UnsafeCell<T>,
    borrowed: Cell<bool>,
}

// SAFETY: this module is compiled only for the single-threaded Wasm guest.
// Dynamic borrowing also excludes aliasing during synchronous host reentry.
unsafe impl<T> Sync for GuestCell<T> {}

impl<T> GuestCell<T> {
    pub(crate) const fn new(value: T) -> Self {
        Self {
            value: UnsafeCell::new(value),
            borrowed: Cell::new(false),
        }
    }

    pub(crate) fn try_borrow_mut(&self) -> Option<GuestBorrow<'_, T>> {
        if self.borrowed.replace(true) {
            None
        } else {
            Some(GuestBorrow { cell: self })
        }
    }
}

pub(crate) struct GuestBorrow<'a, T> {
    cell: &'a GuestCell<T>,
}

impl<'a, T> GuestBorrow<'a, T> {
    pub(crate) fn leak(self) -> &'a mut T {
        let value = self.cell.value.get();
        core::mem::forget(self);
        // SAFETY: the borrow flag stays set forever after forgetting this guard,
        // so no subsequent borrow can alias the returned reference.
        unsafe { &mut *value }
    }
}

impl<T> Deref for GuestBorrow<'_, T> {
    type Target = T;

    fn deref(&self) -> &T {
        // SAFETY: a live guard holds the cell's sole borrow.
        unsafe { &*self.cell.value.get() }
    }
}

impl<T> DerefMut for GuestBorrow<'_, T> {
    fn deref_mut(&mut self) -> &mut T {
        // SAFETY: a live guard holds the cell's sole borrow.
        unsafe { &mut *self.cell.value.get() }
    }
}

impl<T> Drop for GuestBorrow<'_, T> {
    fn drop(&mut self) {
        self.cell.borrowed.set(false);
    }
}

#[inline(never)]
pub(crate) fn copy_slice<T: Copy>(destination: &mut [T], source: &[T]) {
    copy_slice_inline(destination, source);
}

#[inline(always)]
pub(crate) fn copy_slice_inline<T: Copy>(destination: &mut [T], source: &[T]) {
    if destination.len() != source.len() {
        core::arch::wasm32::unreachable();
    }
    // SAFETY: shared and exclusive slice borrows do not overlap, and the
    // checked equal lengths cover the same number of valid T elements.
    unsafe {
        core::ptr::copy_nonoverlapping(source.as_ptr(), destination.as_mut_ptr(), source.len());
    }
}

#[inline(never)]
pub(crate) fn copy_within<T: Copy>(values: &mut [T], start: usize, end: usize, destination: usize) {
    let length = match values.get(start..end) {
        Some(source) => source.len(),
        None => core::arch::wasm32::unreachable(),
    };
    let destination_end = destination
        .checked_add(length)
        .unwrap_or_else(|| core::arch::wasm32::unreachable());
    if values.get(destination..destination_end).is_none() {
        core::arch::wasm32::unreachable();
    }
    // SAFETY: both complete element ranges were checked above; ptr::copy
    // preserves overlapping moves and each element's padding bytes.
    unsafe {
        core::ptr::copy(
            values.as_ptr().add(start),
            values.as_mut_ptr().add(destination),
            length,
        );
    }
}

// The SDK caller decoder copies a fixed slice through a generic panic path.
// Preserve its field, status and identifier checks with a checked array conversion.
#[inline(never)]
pub(crate) fn immediate_caller(
) -> Result<Option<layerx_program_sdk::ProgramId>, layerx_program_sdk::ProgramError> {
    use layerx_program_sdk::{Field, ProgramError, ProgramId, Reason};

    #[link(wasm_import_module = "layerx_v2")]
    unsafe extern "C" {
        fn context_read(field: i32, output_pointer: i32, output_capacity: i32) -> i32;
    }

    let malformed = || ProgramError::value(Field::Buffer, Reason::Malformed);
    let mut output = [0u8; 33];
    let pointer = i32::try_from(output.as_mut_ptr() as usize)
        .map_err(|_| ProgramError::value(Field::Buffer, Reason::TooLarge))?;
    // SAFETY: the frozen context field writes at most 33 bytes into the live,
    // exclusive stack buffer; the pointer fits the ABI's signed integer.
    let length = ProgramError::from_status(unsafe { context_read(2, pointer, 33) })?;
    match (length, output[0]) {
        (1, 0) => Ok(None),
        (33, 1) => {
            let id = output.get(1..).ok_or_else(malformed)?;
            let id = id.try_into().map_err(|_| malformed())?;
            Ok(Some(ProgramId::new(id)?))
        }
        _ => Err(malformed()),
    }
}
