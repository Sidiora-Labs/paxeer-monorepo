use std::collections::HashMap;

use parking_lot::RwLock;
use thiserror::Error;

use crate::core::types::User;

#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum UserStoreError {
    #[error("referral code is already assigned to another user")]
    DuplicateReferralCode,
}

/// In-memory user store, replacing PostgreSQL `users` table.
///
/// Primary key: `address: [u8; 20]`.
/// Secondary index: `referral_code` → `address` (for referral lookups).
///
/// A single lock binds each user to its current referral index entry.
pub struct UserStore {
    inner: RwLock<UserIndexes>,
}

#[derive(Default)]
struct UserIndexes {
    users: HashMap<[u8; 20], User>,
    by_referral_code: HashMap<String, [u8; 20]>,
}

impl UserIndexes {
    fn validate(&self, user: &User) -> Result<(), UserStoreError> {
        if let Some(code) = user.referral_code.as_ref().filter(|code| !code.is_empty()) {
            if self
                .by_referral_code
                .get(code)
                .is_some_and(|owner| owner != &user.address)
            {
                return Err(UserStoreError::DuplicateReferralCode);
            }
        }
        Ok(())
    }

    fn remove_code(&mut self, user: &User) {
        if let Some(code) = &user.referral_code {
            if self.by_referral_code.get(code) == Some(&user.address) {
                self.by_referral_code.remove(code);
            }
        }
    }

    fn install(&mut self, user: User) {
        if let Some(old) = self.users.remove(&user.address) {
            self.remove_code(&old);
        }
        if let Some(code) = user.referral_code.as_ref().filter(|code| !code.is_empty()) {
            self.by_referral_code.insert(code.clone(), user.address);
        }
        self.users.insert(user.address, user);
    }
}

impl UserStore {
    pub fn new() -> Self {
        Self {
            inner: RwLock::new(UserIndexes::default()),
        }
    }

    /// Insert or replace a user. A collision leaves the existing state intact.
    pub fn upsert(&self, user: User) -> Result<(), UserStoreError> {
        let mut inner = self.inner.write();
        inner.validate(&user)?;
        inner.install(user);
        Ok(())
    }

    /// Get user by address. O(1).
    pub fn get(&self, address: &[u8; 20]) -> Option<User> {
        self.inner.read().users.get(address).cloned()
    }

    /// Update a user and its index together. Returns false for a missing user,
    /// a referral collision or a changed primary address, without installing the update.
    /// The callback runs under the store's write lock and must not reenter the store.
    pub fn update(&self, address: &[u8; 20], f: impl FnOnce(&mut User)) -> bool {
        let mut inner = self.inner.write();
        let Some(mut user) = inner.users.get(address).cloned() else {
            return false;
        };
        f(&mut user);
        if user.address != *address || inner.validate(&user).is_err() {
            return false;
        }
        inner.install(user);
        true
    }

    /// Ensure a user exists. If not, create with defaults (matching PG upsert_user).
    pub fn ensure(&self, address: [u8; 20], now: u64) {
        let mut inner = self.inner.write();
        let user = inner.users.entry(address).or_insert_with(|| User {
            address,
            vault_address: None,
            referral_code: None,
            referred_by: None,
            tier: "bronze".to_string(),
            fee_discount_bps: 0,
            created_at: now,
            first_trade_at: None,
            last_active_at: Some(now),
        });
        user.last_active_at = Some(now);
    }

    /// Look up a user by referral code. O(1).
    pub fn by_referral_code(&self, code: &str) -> Option<User> {
        let inner = self.inner.read();
        let address = inner.by_referral_code.get(code)?;
        inner.users.get(address).cloned()
    }

    /// Set a user's vault address.
    pub fn set_vault(&self, address: &[u8; 20], vault: [u8; 20]) {
        if let Some(user) = self.inner.write().users.get_mut(address) {
            user.vault_address = Some(vault);
        }
    }

    /// Set a user's referral code. Returns false if missing or already owned.
    pub fn set_referral_code(&self, address: &[u8; 20], code: String) -> bool {
        self.update(address, |u| {
            u.referral_code = Some(code);
        })
    }

    /// Set the referrer (referred_by). Returns false if user not found.
    pub fn set_referred_by(&self, address: &[u8; 20], referrer: [u8; 20]) -> bool {
        if let Some(user) = self.inner.write().users.get_mut(address) {
            user.referred_by = Some(referrer);
            true
        } else {
            false
        }
    }

    /// Count users referred by a given address.
    pub fn referral_count(&self, referrer: &[u8; 20]) -> u32 {
        self.inner
            .read()
            .users
            .values()
            .filter(|user| user.referred_by.as_ref() == Some(referrer))
            .count() as u32
    }

    /// Remove a user and only its own referral mapping.
    pub fn remove(&self, address: &[u8; 20]) -> Option<User> {
        let mut inner = self.inner.write();
        let user = inner.users.remove(address)?;
        inner.remove_code(&user);
        Some(user)
    }

    /// Total number of users.
    pub fn count(&self) -> usize {
        self.inner.read().users.len()
    }

    /// All users (for snapshots, diagnostics).
    pub fn all(&self) -> Vec<User> {
        self.inner.read().users.values().cloned().collect()
    }

    /// Check if a user exists.
    pub fn contains(&self, address: &[u8; 20]) -> bool {
        self.inner.read().users.contains_key(address)
    }

    /// Users created within a time range [from, to). For daily stats.
    pub fn created_between(&self, from: u64, to: u64) -> Vec<User> {
        self.inner
            .read()
            .users
            .values()
            .filter(|user| user.created_at >= from && user.created_at < to)
            .cloned()
            .collect()
    }
}

impl Default for UserStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn addr(id: u8) -> [u8; 20] {
        let mut buf = [0u8; 20];
        buf[19] = id;
        buf
    }

    fn make_user(id: u8) -> User {
        User {
            address: addr(id),
            vault_address: None,
            referral_code: None,
            referred_by: None,
            tier: "bronze".to_string(),
            fee_discount_bps: 0,
            created_at: 1700000000,
            first_trade_at: None,
            last_active_at: None,
        }
    }

    #[test]
    fn upsert_and_get() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();

        let user = store.get(&addr(1)).unwrap();
        assert_eq!(user.address, addr(1));
        assert_eq!(user.tier, "bronze");
    }

    #[test]
    fn get_nonexistent() {
        let store = UserStore::new();
        assert!(store.get(&addr(99)).is_none());
    }

    #[test]
    fn ensure_creates_if_missing() {
        let store = UserStore::new();
        store.ensure(addr(1), 1700000000);

        let user = store.get(&addr(1)).unwrap();
        assert_eq!(user.tier, "bronze");
        assert_eq!(user.last_active_at, Some(1700000000));
    }

    #[test]
    fn ensure_touches_last_active() {
        let store = UserStore::new();
        store.ensure(addr(1), 1700000000);
        store.ensure(addr(1), 1700001000);

        let user = store.get(&addr(1)).unwrap();
        assert_eq!(user.last_active_at, Some(1700001000));
        assert_eq!(store.count(), 1);
    }

    #[test]
    fn update_user() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();

        let updated = store.update(&addr(1), |u| {
            u.tier = "silver".to_string();
            u.fee_discount_bps = 10;
        });
        assert!(updated);

        let user = store.get(&addr(1)).unwrap();
        assert_eq!(user.tier, "silver");
        assert_eq!(user.fee_discount_bps, 10);
    }

    #[test]
    fn update_nonexistent() {
        let store = UserStore::new();
        assert!(!store.update(&addr(99), |_| {}));
    }

    #[test]
    fn referral_code_index() {
        let store = UserStore::new();
        let mut user = make_user(1);
        user.referral_code = Some("ABC123".to_string());
        store.upsert(user).unwrap();

        let found = store.by_referral_code("ABC123").unwrap();
        assert_eq!(found.address, addr(1));
        assert!(store.by_referral_code("XYZ999").is_none());
    }

    #[test]
    fn set_referral_code() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();

        assert!(store.set_referral_code(&addr(1), "CODE1".to_string()));

        let found = store.by_referral_code("CODE1").unwrap();
        assert_eq!(found.address, addr(1));
    }

    #[test]
    fn set_referred_by() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();
        store.upsert(make_user(2)).unwrap();

        assert!(store.set_referred_by(&addr(2), addr(1)));

        let user2 = store.get(&addr(2)).unwrap();
        assert_eq!(user2.referred_by, Some(addr(1)));
    }

    #[test]
    fn referral_count() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();

        let mut u2 = make_user(2);
        u2.referred_by = Some(addr(1));
        store.upsert(u2).unwrap();

        let mut u3 = make_user(3);
        u3.referred_by = Some(addr(1));
        store.upsert(u3).unwrap();

        let mut u4 = make_user(4);
        u4.referred_by = Some(addr(2));
        store.upsert(u4).unwrap();

        assert_eq!(store.referral_count(&addr(1)), 2);
        assert_eq!(store.referral_count(&addr(2)), 1);
        assert_eq!(store.referral_count(&addr(99)), 0);
    }

    #[test]
    fn set_vault() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();
        store.set_vault(&addr(1), addr(99));

        let user = store.get(&addr(1)).unwrap();
        assert_eq!(user.vault_address, Some(addr(99)));
    }

    #[test]
    fn remove_user() {
        let store = UserStore::new();
        let mut user = make_user(1);
        user.referral_code = Some("DEL1".to_string());
        store.upsert(user).unwrap();

        let removed = store.remove(&addr(1)).unwrap();
        assert_eq!(removed.address, addr(1));
        assert!(store.get(&addr(1)).is_none());
        assert!(store.by_referral_code("DEL1").is_none());
    }

    #[test]
    fn remove_nonexistent() {
        let store = UserStore::new();
        assert!(store.remove(&addr(99)).is_none());
    }

    #[test]
    fn count() {
        let store = UserStore::new();
        assert_eq!(store.count(), 0);

        store.upsert(make_user(1)).unwrap();
        store.upsert(make_user(2)).unwrap();
        assert_eq!(store.count(), 2);
    }

    #[test]
    fn all_users() {
        let store = UserStore::new();
        store.upsert(make_user(1)).unwrap();
        store.upsert(make_user(2)).unwrap();
        assert_eq!(store.all().len(), 2);
    }

    #[test]
    fn contains() {
        let store = UserStore::new();
        assert!(!store.contains(&addr(1)));
        store.upsert(make_user(1)).unwrap();
        assert!(store.contains(&addr(1)));
    }

    #[test]
    fn created_between() {
        let store = UserStore::new();
        let mut u1 = make_user(1);
        u1.created_at = 100;
        let mut u2 = make_user(2);
        u2.created_at = 200;
        let mut u3 = make_user(3);
        u3.created_at = 300;
        store.upsert(u1).unwrap();
        store.upsert(u2).unwrap();
        store.upsert(u3).unwrap();

        let range = store.created_between(100, 300);
        assert_eq!(range.len(), 2);
    }

    #[test]
    fn referral_code_update_reindexes() {
        let store = UserStore::new();
        let mut user = make_user(1);
        user.referral_code = Some("OLD".to_string());
        store.upsert(user).unwrap();

        assert!(store.by_referral_code("OLD").is_some());

        store.update(&addr(1), |u| {
            u.referral_code = Some("NEW".to_string());
        });

        assert!(store.by_referral_code("OLD").is_none());
        assert!(store.by_referral_code("NEW").is_some());
    }
}
