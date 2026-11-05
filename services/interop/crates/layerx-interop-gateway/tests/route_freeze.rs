use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::error::GatewayError;
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::server::{
    interop_gateway_routes, ExternalState, HostedAdapter, IngressTransport, InteropRoute,
};
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;

const FROZEN_ROUTES: [(&str, &str, InteropRoute<'static>); 9] = [
    (
        "POST",
        "/v1/http/x402/buyer/build",
        InteropRoute::X402BuyerBuild {
            transport: IngressTransport::Http,
        },
    ),
    (
        "POST",
        "/v1/http/x402/verify",
        InteropRoute::X402Verify {
            transport: IngressTransport::Http,
        },
    ),
    (
        "POST",
        "/v1/http/x402/settle",
        InteropRoute::X402Settle {
            transport: IngressTransport::Http,
        },
    ),
    ("POST", "/v1/http/ap2/execute", InteropRoute::Ap2Execute),
    (
        "POST",
        "/v1/http/ucp/checkouts/complete",
        InteropRoute::UcpComplete,
    ),
    (
        "POST",
        "/v1/http/visa-tap/intents/execute",
        InteropRoute::VisaExecuteIntent,
    ),
    (
        "POST",
        "/v1/http/fiat/card/callbacks",
        InteropRoute::FiatCallback {
            adapter: HostedAdapter::FiatCard,
        },
    ),
    (
        "POST",
        "/v1/http/fiat/bank/callbacks",
        InteropRoute::FiatCallback {
            adapter: HostedAdapter::FiatBank,
        },
    ),
    (
        "POST",
        "/v1/http/fiat/rtp/callbacks",
        InteropRoute::FiatCallback {
            adapter: HostedAdapter::FiatRtp,
        },
    ),
];

#[test]
fn settling_routes_keep_their_paths_and_methods() {
    for (method, path, expected) in &FROZEN_ROUTES {
        assert_eq!(
            interop_gateway_routes(method, path),
            Ok(expected.clone()),
            "{method} {path}"
        );
        for other in ["GET", "HEAD", "PUT", "PATCH", "DELETE", "OPTIONS", "post"] {
            assert!(
                interop_gateway_routes(other, path).is_err(),
                "unexpected method {other} for {path}"
            );
        }
        for changed in [format!("{path}/"), format!("{path}?settle=true")] {
            assert!(interop_gateway_routes(method, &changed).is_err());
        }
    }

    for (label, transport) in [
        ("http", IngressTransport::Http),
        ("mcp", IngressTransport::Mcp),
        ("a2a", IngressTransport::A2a),
    ] {
        for (tail, expected) in [
            ("buyer/build", InteropRoute::X402BuyerBuild { transport }),
            ("verify", InteropRoute::X402Verify { transport }),
            ("facilitator/verify", InteropRoute::X402Verify { transport }),
            ("settle", InteropRoute::X402Settle { transport }),
            ("seller/settle", InteropRoute::X402Settle { transport }),
            ("facilitator/settle", InteropRoute::X402Settle { transport }),
        ] {
            let path = format!("/v1/{label}/x402/{tail}");
            assert_eq!(interop_gateway_routes("POST", &path), Ok(expected));
            assert!(interop_gateway_routes("GET", &path).is_err());
        }
        if transport != IngressTransport::Http {
            for (_, path, _) in &FROZEN_ROUTES[3..] {
                let path = path.replacen("/http/", &format!("/{label}/"), 1);
                assert!(interop_gateway_routes("POST", &path).is_err());
            }
        }
    }
}

#[test]
fn receipt_pending_answers_submitted_with_202() {
    let trace = TraceId::mint([0x11; 16]);
    let principal = PrincipalId::new("route-freeze-buyer").unwrap();
    let key = [0x31; 32];
    let evidence = SequencerEvidence {
        transaction_hash: [0x41; 32],
        signed_receipt: Vec::new(),
        logs: Vec::new(),
        rung: SettlementRung::Submitted,
    };

    for (method, path, expected) in &FROZEN_ROUTES {
        if !expected.state_changing() {
            continue;
        }
        let route = interop_gateway_routes(method, path).unwrap();
        let adapter = AdapterId::new(route.adapter().unwrap().id()).unwrap();
        let spec = PinnedSpec::new(
            adapter.clone(),
            SpecVersion::parse("1.0.0").unwrap(),
            [0x51; 32],
        )
        .unwrap();
        let suite = ConformanceSuite::new(
            AdapterId::new("route-freeze-vectors").unwrap(),
            2,
            [0x52; 32],
        )
        .unwrap();
        let mut core = GatewayCore::new();
        core.register_adapter(
            AdapterDescriptor::new(adapter.clone(), spec, suite),
            &trace,
            1,
        )
        .unwrap();
        let request =
            TranslationRequest::new(adapter, TranslationKind::StateChanging, key, [0x42; 32])
                .unwrap();
        assert_eq!(
            core.begin_translation(&principal, &request, &trace, 2)
                .unwrap(),
            TranslationStatus::Pending
        );
        let before = format!("{core:?}");

        for now in [3, 4] {
            let error = core
                .settle_with_sequencer_evidence(&principal, key, &evidence, &[], &trace, now)
                .unwrap_err();
            assert_eq!(error.trace(), &trace);
            let SequencerSettlementError::Gateway(error) = *error.error() else {
                panic!("submitted settlement must return a gateway error");
            };
            assert_eq!(error, GatewayError::ReceiptPending);
            let (status, state) = ExternalState::settlement_error_response(error).unwrap();
            assert_eq!(status, 202, "{path}");
            assert_eq!(state, ExternalState::Submitted, "{path}");
            assert_eq!(state.label(), "submitted", "{path}");
            assert_eq!(request.idempotency_key(), key);
            assert_eq!(
                core.begin_translation(&principal, &request, &trace, now)
                    .unwrap(),
                TranslationStatus::Pending
            );
            assert_eq!(
                core.translation(&principal, key),
                Some(TranslationStatus::Pending)
            );
            assert_eq!(format!("{core:?}"), before);
        }
    }

    for error in [
        GatewayError::ReceiptRequired,
        GatewayError::IdempotencyConflict,
        GatewayError::UnknownTranslation,
        GatewayError::TranslationClosed,
    ] {
        assert_eq!(ExternalState::settlement_error_response(error), Err(error));
    }
    for (state, label) in [
        (ExternalState::Pending, "pending"),
        (ExternalState::ReversalPending, "reversal-pending"),
        (ExternalState::Refused, "refused"),
        (ExternalState::ReceiptVerified, "receipt-verified"),
        (ExternalState::Reversed, "reversed"),
    ] {
        assert_eq!(state.label(), label);
    }
}
