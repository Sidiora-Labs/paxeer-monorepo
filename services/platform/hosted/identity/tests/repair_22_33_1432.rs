#[path = "support/signup_service.rs"]
mod signup_service;

#[test]
fn repair_22_33_1432_signup_delivery_session_and_credit_survive_restarts() {
    signup_service::signup_delivery_session_and_credit_survive_restarts();
}

#[test]
fn repair_22_33_1432_expired_delivered_token_is_refused_after_restart() {
    signup_service::expired_delivered_token_is_refused_after_restart();
}

#[test]
fn repair_22_33_1432_interrupted_smtp_delivery_retries_the_durable_token() {
    signup_service::interrupted_smtp_delivery_retries_the_durable_token();
}

#[test]
fn repair_22_33_1432_interrupted_outbox_enqueue_never_verifies_the_subject() {
    signup_service::interrupted_outbox_enqueue_never_verifies_the_subject();
}
