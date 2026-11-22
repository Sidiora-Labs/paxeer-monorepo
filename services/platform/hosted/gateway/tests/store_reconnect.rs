#[path = "support/redis_process.rs"]
mod redis_process;
use redis_process::RedisProcess;

#[test]
fn store_reconnects_on_the_first_ready_call_after_server_restart() {
    let mut redis = RedisProcess::start();
    let store = redis.store();
    assert!(store.ready());
    assert!(store.ready());

    let process_id = redis.process_id();
    redis.restart();
    assert_ne!(redis.process_id(), process_id);

    assert!(store.ready());
    assert!(store.ready());
}
