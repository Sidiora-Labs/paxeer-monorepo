use super::SourceAggregator;
use crate::sources::{precompiles::PrecompileSource, MockSignatureSource};
use std::sync::Arc;

#[tokio::test]
async fn precompile_signature_ranks_first() {
    let precompile_signature = "cancelOrder(bytes32)";
    let colliding_signature = "cancelOrder(uint256)";
    let selector = hex::encode(ethabi::short_signature(
        "cancelOrder",
        &[ethabi::ParamType::FixedBytes(32)],
    ));

    let mut source = MockSignatureSource::new();
    source
        .expect_get_function_signatures()
        .with(mockall::predicate::eq(selector.clone()))
        .times(1)
        .returning(move |_| {
            Ok(vec![
                colliding_signature.to_owned(),
                precompile_signature.to_owned(),
            ])
        });

    let aggregator = SourceAggregator::new(
        vec![Arc::new(PrecompileSource::new()), Arc::new(source)],
        vec![],
    );
    let signatures = aggregator.get_function_signatures(&selector).await.unwrap();

    assert_eq!(
        signatures,
        vec![
            precompile_signature.to_owned(),
            colliding_signature.to_owned(),
        ]
    );
}
