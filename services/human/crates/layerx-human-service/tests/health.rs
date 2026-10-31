use layerx_human_service::health::{
    ComponentHealth, HealthComponent, ServiceHealth, ServiceReadiness,
};

fn ready_components() -> ServiceHealth {
    ServiceHealth {
        human_service: ComponentHealth::Ready,
        agent_layer: ComponentHealth::Ready,
    }
}

#[test]
fn service_and_agent_failures_are_reported_separately() {
    let mut health = ready_components();
    assert_eq!(
        health.readiness(),
        ServiceReadiness {
            ready: true,
            unavailable: Vec::new(),
        }
    );

    health.human_service = ComponentHealth::Unavailable {
        reason: "store unavailable".to_owned(),
    };
    health.agent_layer = ComponentHealth::Unavailable {
        reason: "agent boundary unavailable".to_owned(),
    };

    assert_eq!(
        health.readiness(),
        ServiceReadiness {
            ready: false,
            unavailable: vec![HealthComponent::HumanService, HealthComponent::AgentLayer],
        }
    );
}
