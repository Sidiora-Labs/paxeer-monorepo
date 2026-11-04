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
