use layerx_agentd::budget::{budget_state_is_committed, BudgetVerificationError};

#[path = "human_approval_budget_actor.rs"]
mod approval_fixture;

#[test]
fn committed_budget_label_is_accepted() {
    assert_eq!(budget_state_is_committed(6), Ok(()));
}

#[test]
fn current_lower_budget_levels_are_not_committed() {
    for label in 0..=3 {
        assert_eq!(
            budget_state_is_committed(label),
            Err(BudgetVerificationError::NotCommitted),
            "label {label}"
        );
    }
}

#[test]
fn retired_and_every_unknown_budget_label_are_refused() {
    for label in [4, 5].into_iter().chain(7..=u8::MAX) {
        assert_eq!(
            budget_state_is_committed(label),
            Err(BudgetVerificationError::RetiredOrUnknown),
            "label {label}"
        );
    }
}
