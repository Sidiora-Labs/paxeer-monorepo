#[allow(
    dead_code,
    reason = "The host binary uses these signalling and deal items; this test target exercises a subset."
)]
mod signal {
    include!("../src/inference_market/signal.rs");

    pub mod deals {
        include!("../src/inference_market/deals.rs");
    }

    pub(super) fn receive_signal(
        stream: &mut TcpStream,
        writer: &Arc<Mutex<TcpStream>>,
    ) -> Result<ClientFrame, SignalError> {
        receive(stream, writer, &mut None)
    }
}

use std::any::type_name;
use std::fmt::Debug;
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::Value;
use signal::deals::{DealAcknowledgement, DealServerFrame};
use signal::{HelloFrame, SignalError};
use tungstenite::{protocol::Role, Message, WebSocket};

type ClientFrame = signal::ClientFrame;
type ServerFrame = signal::ServerFrame;
type DealClientFrame = signal::deals::DealClientFrame;

#[derive(Deserialize)]
struct Fixtures {
    key: FixtureKey,
    vectors: Vec<Vector>,
}

#[derive(Deserialize)]
struct FixtureKey {
    peer_key_hex: String,
}

#[derive(Deserialize)]
struct Vector {
    name: String,
    direction: Direction,
    json: String,
    signing_bytes_hex: Option<String>,
    signature_hex: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "lowercase")]
enum Direction {
    Client,
    Server,
}

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("unexpected failure: {error:?}"))
}

fn fixtures() -> Fixtures {
    must(serde_json::from_str(include_str!(
        "../../../../../apps/inference-peer/test/fixtures/host-signal-frames.json"
    )))
}

impl Fixtures {
    fn vector(&self, name: &str) -> &Vector {
        self.vectors
            .iter()
            .find(|vector| vector.name == name)
            .unwrap_or_else(|| panic!("missing peer vector {name}"))
    }
}

impl Vector {
    fn decode<T: DeserializeOwned>(&self) -> T {
        serde_json::from_str(&self.json).unwrap_or_else(|error| {
            panic!(
                "peer vector {} refused by {}: {error}; vector: {}",
                self.name,
                type_name::<T>(),
                self.json
            )
        })
    }

    fn assert_round_trip<T: Serialize>(&self, frame: &T) {
        let actual: Value = must(serde_json::from_str(&must(serde_json::to_string(frame))));
        assert_eq!(actual, self.decode::<Value>(), "vector {}", self.name);
    }

    fn verify_signature(&self, key: &[u8; 32], bytes: &[u8], signature: &[u8]) {
        assert_eq!(
            Some(hex::encode(bytes)).as_ref(),
            self.signing_bytes_hex.as_ref(),
            "signing bytes of {}",
            self.name
        );
        assert_eq!(
            Some(hex::encode(signature)).as_ref(),
            self.signature_hex.as_ref(),
            "signature of {}",
            self.name
        );
        must(layerx_crypto::ed25519::verify_message(
            key,
            &must(signature.try_into()),
            bytes,
        ));
    }
}

#[test]
fn host_types_read_every_vector_of_the_peer() {
    let fixtures = fixtures();
    assert_eq!(
        fixtures
            .vectors
            .iter()
            .map(|vector| vector.name.as_str())
            .collect::<Vec<_>>(),
        [
            "challenge",
            "hello",
            "offer",
            "description_to",
            "description_from",
            "candidate_to",
            "candidate_from",
            "deal",
            "deal_ack",
            "deal_live",
            "error",
        ]
    );
    for vector in &fixtures.vectors {
        match vector.direction {
            Direction::Client => match vector.decode::<DealClientFrame>() {
                DealClientFrame::Acknowledgement(frame) => vector.assert_round_trip(&frame),
                DealClientFrame::Signal(frame) => {
                    vector.assert_round_trip(&frame);
                    vector.assert_round_trip(&vector.decode::<ClientFrame>());
                }
            },
            Direction::Server => match vector.name.as_str() {
                "deal" | "deal_live" => {
                    vector.assert_round_trip(&vector.decode::<DealServerFrame>());
                }
                _ => vector.assert_round_trip(&vector.decode::<ServerFrame>()),
            },
        }
    }
}

#[test]
fn hello_and_acknowledgement_signatures_verify() {
    let fixtures = fixtures();
    let key: [u8; 32] = must(must(hex::decode(&fixtures.key.peer_key_hex)).try_into());
    let hello_vector = fixtures.vector("hello");
    let ClientFrame::Hello(hello) = hello_vector.decode::<ClientFrame>() else {
        panic!("hello vector is not ClientFrame::Hello");
    };
    let ServerFrame::Challenge { nonce } = fixtures.vector("challenge").decode::<ServerFrame>()
    else {
        panic!("challenge vector is not ServerFrame::Challenge");
    };
    assert_eq!(hello.peer_key, key);
    assert_eq!(hello.nonce, nonce);
    hello_vector.verify_signature(
        &key,
        &HelloFrame::signing_bytes(&hello.room_id, &nonce),
        &hello.signature,
    );

    let DealServerFrame::Deal { deal } = fixtures.vector("deal").decode::<DealServerFrame>() else {
        panic!("deal vector is not DealServerFrame::Deal");
    };
    let ack_vector = fixtures.vector("deal_ack");
    let DealClientFrame::Acknowledgement(DealAcknowledgement::DealAck {
        deal_id,
        digest,
        signature,
    }) = ack_vector.decode::<DealClientFrame>()
    else {
        panic!("deal_ack vector is not DealClientFrame::Acknowledgement");
    };
    assert_eq!(deal_id, deal.deal_id);
    assert_eq!(digest, must(deal.digest()));
    ack_vector.verify_signature(&key, &must(deal.acknowledgement_bytes()), &signature);
    ack_vector.assert_round_trip(&must(
        deal.acknowledgement(&key, must(signature.as_slice().try_into())),
    ));
}

fn receive_frame(message: Message, deal: bool) -> Result<(), SignalError> {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let client = must(TcpStream::connect(must(listener.local_addr())));
    let (mut stream, _) = must(listener.accept());
    must(stream.set_read_timeout(Some(Duration::from_secs(3))));
    must(stream.set_write_timeout(Some(Duration::from_secs(3))));
    let writer = Arc::new(Mutex::new(must(stream.try_clone())));
    let mut socket = WebSocket::from_raw_socket(client, Role::Client, None);
    must(socket.send(message));
    if deal {
        signal::deals::receive_deal(&mut stream, &writer, &mut None).map(|_| ())
    } else {
        signal::receive_signal(&mut stream, &writer).map(|_| ())
    }
}

#[test]
fn a_frame_of_the_retired_form_is_refused() {
    let fixtures = fixtures();
    let hello = fixtures.vector("hello");
    for deal in [false, true] {
        assert_eq!(
            receive_frame(Message::Text(hello.json.clone().into()), deal),
            Ok(()),
            "the original hello must be accepted (deal listener: {deal})"
        );
        assert_eq!(
            receive_frame(Message::Binary(hello.json.as_bytes().to_vec().into()), deal),
            Err(SignalError::InvalidFrame),
            "binary signalling must be refused (deal listener: {deal})"
        );
    }
    let mut unknown = hello.decode::<Value>();
    unknown["future"] = Value::Bool(true);
    let Err(error) = serde_json::from_value::<ClientFrame>(unknown.clone()) else {
        panic!("ClientFrame accepted an unknown hello member");
    };
    assert!(
        error.to_string().contains("unknown field `future`"),
        "{error}"
    );
    assert!(serde_json::from_value::<DealClientFrame>(unknown.clone()).is_err());
    for deal in [false, true] {
        assert_eq!(
            receive_frame(Message::Text(unknown.to_string().into()), deal),
            Err(SignalError::InvalidFrame),
            "unknown hello members must be refused (deal listener: {deal})"
        );
    }
}
