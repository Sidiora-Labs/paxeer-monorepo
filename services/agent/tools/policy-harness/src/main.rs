use layerx_policy_harness::agent_policy_harness;

fn main() {
    let report = agent_policy_harness().unwrap_or_else(|error| panic!("policy harness: {error}"));
    for decision in &report.decisions {
        println!(
            "case={} outcome={:?} reason={:?} policy_version={} matched_rules={:?} deciding_rule={:?} dimension={:?}",
            decision.case, decision.outcome, decision.reason, decision.policy_version,
            decision.matched_rules, decision.deciding_rule, decision.dimension,
        );
    }
    println!(
        "cases={} constraints_blocked={} observed_height={} protocol_consumed={}",
        report.decisions.len(),
        report.blocked_constraints.len(),
        report.observed_height,
        report.protocol_consumed,
    );
}

#[cfg(test)]
mod tests {
    use layerx_agentd::policy::{evaluate, DecisionReason, EvaluationInput, Outcome};
    use layerx_policy_harness::adversarial::agent_policy_adversarial_corpus;

    #[test]
    fn unavailable_budget_denies_the_complete_adversarial_corpus() {
        let decisions: Vec<_> = agent_policy_adversarial_corpus()
            .iter()
            .map(|case| {
                let input = EvaluationInput::without_protocol_budget(
                    &case.request,
                    &case.session,
                    &case.capability,
                );
                evaluate(&case.policy, &input)
            })
            .collect();
        assert!(decisions
            .iter()
            .all(|decision| decision.outcome == Outcome::Deny));
        assert!(decisions
            .iter()
            .all(|decision| decision.reason == DecisionReason::InvalidContext));
        assert_eq!(
            decisions
                .iter()
                .filter(|decision| decision.outcome == Outcome::Allow)
                .count(),
            0
        );
    }
}
