use actix_prost_build::{ActixGenerator, GeneratorList};
use prost_build::{Config, ServiceGenerator};
use std::path::Path;

// custom function to include custom generator
fn compile(
    protos: &[impl AsRef<Path>],
    includes: &[impl AsRef<Path>],
    generator: Box<dyn ServiceGenerator>,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut config = Config::new();
    config
        .service_generator(generator)
        .compile_well_known_types()
        .bytes(["."])
        .type_attribute(".", "#[actix_prost_macros::serde]")
        .field_attribute(
            ".blockscout.sig_provider.v1.HealthCheckRequest.service",
            "#[serde(default)]",
        )
        .message_attribute(".", "#[derive(Eq, Hash)]");
    config.compile_protos(protos, includes)?;
    Ok(())
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    std::env::set_var("PROTOC", protoc_bin_vendored::protoc_bin_path()?);
    // We need to rebuild proto lib only if any of proto definitions
    // (or corresponding http mapping) has been changed.
    println!("cargo:rerun-if-changed=proto/");

    let gens = Box::new(GeneratorList::new(vec![
        tonic_build::configure().service_generator(),
        Box::new(ActixGenerator::new("proto/api_config_http.yaml").unwrap()),
    ]));
    compile(
        &["proto/sig-provider.proto", "proto/health.proto"],
        &["proto"],
        gens,
    )?;
    Ok(())
}
