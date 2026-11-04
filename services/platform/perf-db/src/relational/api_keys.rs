use std::collections::HashMap;

use parking_lot::RwLock;
use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApiKey {
    pub id: String,
    pub address: [u8; 20],
    pub key_hash: String,
    pub label: Option<String>,
    pub rate_limit_rpm: u32,
    pub last_used_at: Option<u64>,
    pub expires_at: Option<u64>,
    pub created_at: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum ApiKeyError {
    #[error("API key hash is already assigned to another key")]
    DuplicateHash,
}

pub struct ApiKeyStore {
    inner: RwLock<ApiKeyIndexes>,
}

#[derive(Default)]
struct ApiKeyIndexes {
    primary: HashMap<String, ApiKey>,
    by_user: HashMap<[u8; 20], Vec<String>>,
    by_hash: HashMap<String, String>,
}

impl ApiKeyIndexes {
    fn remove(&mut self, id: &str) -> Option<ApiKey> {
        let key = self.primary.remove(id)?;
        if self
            .by_hash
            .get(&key.key_hash)
            .is_some_and(|owner| owner == id)
        {
            self.by_hash.remove(&key.key_hash);
        }
        if let Some(ids) = self.by_user.get_mut(&key.address) {
            ids.retain(|old| old != id);
            if ids.is_empty() {
                self.by_user.remove(&key.address);
            }
        }
        Some(key)
    }
}

impl ApiKeyStore {
    pub fn new() -> Self {
        Self {
            inner: RwLock::new(ApiKeyIndexes::default()),
        }
    }

    pub fn insert(&self, key: ApiKey) -> Result<(), ApiKeyError> {
        let mut inner = self.inner.write();
        if inner
            .by_hash
            .get(&key.key_hash)
            .is_some_and(|id| id != &key.id)
        {
            return Err(ApiKeyError::DuplicateHash);
        }
        inner.remove(&key.id);
        inner.by_hash.insert(key.key_hash.clone(), key.id.clone());
        inner
            .by_user
            .entry(key.address)
            .or_default()
            .push(key.id.clone());
        inner.primary.insert(key.id.clone(), key);
        Ok(())
    }

    pub fn get(&self, id: &str) -> Option<ApiKey> {
        self.inner.read().primary.get(id).cloned()
    }

    pub fn get_by_hash(&self, hash: &str) -> Option<ApiKey> {
        let inner = self.inner.read();
        let id = inner.by_hash.get(hash)?;
        inner.primary.get(id).cloned()
    }

    pub fn by_user(&self, address: &[u8; 20]) -> Vec<ApiKey> {
        let inner = self.inner.read();
        inner
            .by_user
            .get(address)
            .into_iter()
            .flatten()
            .filter_map(|id| inner.primary.get(id).cloned())
            .collect()
    }

    pub fn remove(&self, id: &str) -> Option<ApiKey> {
        self.inner.write().remove(id)
    }

    pub fn remove_by_user_and_id(&self, address: &[u8; 20], id: &str) -> Option<ApiKey> {
        let mut inner = self.inner.write();
        if inner.primary.get(id)?.address != *address {
            return None;
        }
        inner.remove(id)
    }

    pub fn touch_last_used(&self, id: &str, timestamp: u64) -> bool {
        let mut inner = self.inner.write();
        if let Some(key) = inner.primary.get_mut(id) {
            key.last_used_at = Some(timestamp);
            true
        } else {
            false
        }
    }

    pub fn all(&self) -> Vec<ApiKey> {
        self.inner.read().primary.values().cloned().collect()
    }

    pub fn count(&self) -> usize {
        self.inner.read().primary.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_addr(n: u8) -> [u8; 20] {
        let mut a = [0u8; 20];
        a[19] = n;
        a
    }

    fn make_key(addr: [u8; 20], id: &str, hash: &str) -> ApiKey {
        ApiKey {
            id: id.to_string(),
            address: addr,
            key_hash: hash.to_string(),
            label: Some("test".to_string()),
            rate_limit_rpm: 60,
            last_used_at: None,
            expires_at: None,
            created_at: 1000,
        }
    }

    #[test]
    fn insert_and_get() {
        let store = ApiKeyStore::new();
        store
            .insert(make_key(test_addr(1), "uuid-1", "hash-1"))
            .unwrap();
        let key = store.get("uuid-1").unwrap();
        assert_eq!(key.key_hash, "hash-1");
    }

    #[test]
    fn get_by_hash() {
        let store = ApiKeyStore::new();
        store
            .insert(make_key(test_addr(1), "uuid-1", "hash-abc"))
            .unwrap();
        let key = store.get_by_hash("hash-abc").unwrap();
        assert_eq!(key.id, "uuid-1");
    }

    #[test]
    fn by_user() {
        let store = ApiKeyStore::new();
        let addr = test_addr(1);
        store.insert(make_key(addr, "k1", "h1")).unwrap();
        store.insert(make_key(addr, "k2", "h2")).unwrap();
        store.insert(make_key(test_addr(2), "k3", "h3")).unwrap();
        assert_eq!(store.by_user(&addr).len(), 2);
    }

    #[test]
    fn remove() {
        let store = ApiKeyStore::new();
        store
            .insert(make_key(test_addr(1), "uuid-1", "hash-1"))
            .unwrap();
        assert!(store.remove("uuid-1").is_some());
        assert!(store.get("uuid-1").is_none());
        assert!(store.get_by_hash("hash-1").is_none());
    }

    #[test]
    fn remove_wrong_user() {
        let store = ApiKeyStore::new();
        store
            .insert(make_key(test_addr(1), "uuid-1", "hash-1"))
            .unwrap();
        assert!(
            store
                .remove_by_user_and_id(&test_addr(2), "uuid-1")
                .is_none()
        );
        assert!(store.get("uuid-1").is_some());
    }

    #[test]
    fn touch_last_used() {
        let store = ApiKeyStore::new();
        store
            .insert(make_key(test_addr(1), "uuid-1", "hash-1"))
            .unwrap();
        assert!(store.touch_last_used("uuid-1", 5000));
        assert_eq!(store.get("uuid-1").unwrap().last_used_at, Some(5000));
    }
}
