#[path = "../support/application_proof.rs"]
mod application_proof;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    application_proof::run(std::io::stdin().lock(), std::io::stdout().lock())
}
