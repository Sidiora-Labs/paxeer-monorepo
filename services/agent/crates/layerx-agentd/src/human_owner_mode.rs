use std::env;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Mode {
    Full,
    HumanOwner,
    InferenceMarket,
}

impl Mode {
    pub fn parse(value: Option<&str>) -> Result<Self, String> {
        match value {
            None | Some("full") => Ok(Self::Full),
            Some("human-owner") => Ok(Self::HumanOwner),
            Some("inference-market") => Ok(Self::InferenceMarket),
            Some(_) => Err("LAYERX_AGENT_MODE is invalid".to_owned()),
        }
    }

    pub fn from_environment() -> Result<Self, String> {
        match env::var("LAYERX_AGENT_MODE") {
            Ok(value) => Self::parse(Some(&value)),
            Err(env::VarError::NotPresent) => Self::parse(None),
            Err(env::VarError::NotUnicode(_)) => Err("LAYERX_AGENT_MODE is invalid".to_owned()),
        }
    }
}

pub fn run(
    full: impl FnOnce() -> Result<(), String>,
    human_owner: impl FnOnce() -> Result<(), String>,
    inference_market: impl FnOnce() -> Result<(), String>,
) -> Result<(), String> {
    match Mode::from_environment()? {
        Mode::Full => full(),
        Mode::HumanOwner => human_owner(),
        Mode::InferenceMarket => inference_market(),
    }
}

#[cfg(test)]
mod tests {
    use super::Mode;

    #[test]
    fn mode_selection_is_explicit_and_closed() {
        assert_eq!(Mode::parse(None), Ok(Mode::Full));
        assert_eq!(Mode::parse(Some("full")), Ok(Mode::Full));
        assert_eq!(Mode::parse(Some("human-owner")), Ok(Mode::HumanOwner));
        for value in ["", "human", "FULL", "human-owner "] {
            assert!(Mode::parse(Some(value)).is_err());
        }
    }
}
