mod control {
    include!("../src/main.rs");

    const README: &str = include_str!("../README.md");
    const SMOKE: &str = include_str!("hosted-smoke.sh");
    const PAYMENT: &str = include_str!("payment-journey.py");
    const INSPECTION: &str = include_str!("receipt_inspection.py");

    fn documented_rows(header: &str) -> Vec<Vec<String>> {
        README
            .split_once(header)
            .unwrap_or_else(|| panic!("missing README table {header}"))
            .1
            .lines()
            .skip(2)
            .take_while(|line| line.starts_with('|'))
            .map(|line| {
                line.trim_matches('|')
                    .split('|')
                    .map(|cell| cell.replace('`', "").trim().to_owned())
                    .collect()
            })
            .collect()
    }

    fn round_trip(response: Response) -> Result<(u16, serde_json::Value), String> {
        let listener = TcpListener::bind("127.0.0.1:0").map_err(|e| e.to_string())?;
        let mut client = TcpStream::connect(listener.local_addr().map_err(|e| e.to_string())?)
            .map_err(|e| e.to_string())?;
        client
            .set_read_timeout(Some(IO_TIMEOUT))
            .map_err(|e| e.to_string())?;
        client
            .set_write_timeout(Some(IO_TIMEOUT))
            .map_err(|e| e.to_string())?;
        thread::scope(|scope| {
            let server = scope.spawn(move || -> Result<(), String> {
                let (mut connection, _) = listener.accept().map_err(|e| e.to_string())?;
                connection
                    .set_read_timeout(Some(IO_TIMEOUT))
                    .map_err(|e| e.to_string())?;
                connection
                    .set_write_timeout(Some(IO_TIMEOUT))
                    .map_err(|e| e.to_string())?;
                let request = client_request(&mut connection)?;
                assert_eq!(request.method, "GET");
                assert_eq!(request.path, "/readyz");
                write_response(&mut connection, &response)
            });
            client
                .write_all(b"GET /readyz HTTP/1.1\r\nHost: localhost\r\n\r\n")
                .map_err(|e| e.to_string())?;
            let message = read_message(&mut client)?;
            server
                .join()
                .map_err(|_| "HTTP producer panicked".to_owned())??;
            let status = message.headers[""]
                .split_whitespace()
                .nth(1)
                .ok_or("missing HTTP status")?
                .parse::<u16>()
                .map_err(|e| e.to_string())?;
            assert_eq!(message.headers["content-type"], "application/json");
            assert_eq!(message.headers["cache-control"], "no-store");
            let document = serde_json::from_slice(&message.body).map_err(|e| e.to_string())?;
            Ok((status, document))
        })
    }

    #[test]
    fn doc_testnet_readme_control_contract_round_trip() -> Result<(), String> {
        let _ = main as fn();
        let release = ReleaseGate {
            package_semver: env!("CARGO_PKG_VERSION").to_owned(),
            pending_package_semver: env!("CARGO_PKG_VERSION").to_owned(),
            pending_wire_version: LXP_WIRE_PROTOCOL_VERSION,
        };
        let dependency_rows = documented_rows("| Dependency | Service and manifest |");
        let journey_rows =
            documented_rows("| Admission endpoint | Status journey | Dependencies |");
        assert_eq!(dependency_rows.len(), 8);
        assert_eq!(journey_rows.len(), 3);
        for failed in std::iter::once(None).chain(Dependency::ALL.into_iter().map(Some)) {
            // Peer probe outcomes feed the unchanged production readiness producer.
            let reports = DependencyReports::new(|dependency| {
                if Some(dependency) == failed {
                    Err("GET /readyz unavailable".to_owned())
                } else {
                    Ok("GET /readyz answered 200".to_owned())
                }
            });
            let readiness = HostedReadiness::assemble(&reports, &release);
            let (status, document) = round_trip(json_response(
                if readiness.is_ready() { 200 } else { 503 },
                readiness.document(&release),
            ))?;
            assert_eq!(status, if failed.is_none() { 200 } else { 503 });
            assert_eq!(
                document["state"],
                if failed.is_none() {
                    "ready"
                } else {
                    "degraded"
                }
            );
            let dependencies = document["dependencies"]
                .as_array()
                .ok_or("missing dependencies")?;
            assert_eq!(dependencies.len(), dependency_rows.len());
            for (entry, row) in dependencies.iter().zip(&dependency_rows) {
                assert_eq!(entry["name"], row[0]);
                assert_eq!(
                    entry["ready"],
                    failed.is_none_or(|dependency| dependency.name() != row[0])
                );
            }
            let journeys = document["journeys"].as_array().ok_or("missing journeys")?;
            assert_eq!(journeys.len(), journey_rows.len());
            for ((entry, row), journey) in journeys.iter().zip(&journey_rows).zip(Journey::ALL) {
                assert_eq!(Journey::from_route(&row[0]), Some(journey));
                assert_eq!(entry["journey"], row[1]);
                let names: Vec<_> = entry["dependencies"]
                    .as_array()
                    .ok_or("missing journey dependencies")?
                    .iter()
                    .map(|dependency| dependency["name"].as_str().ok_or("missing dependency name"))
                    .collect::<Result<_, _>>()?;
                assert_eq!(names.join(", "), row[2]);
                let ready =
                    failed.is_none_or(|dependency| !journey.dependencies().contains(&dependency));
                let (admission_status, admission) = round_trip(json_response(
                    if ready { 200 } else { 503 },
                    JourneyReadiness::compute(journey, &reports).admission(),
                ))?;
                assert_eq!(admission_status, if ready { 200 } else { 503 });
                assert_eq!(admission["admitted"], ready);
                assert_eq!(admission["ready"], ready);
                assert_eq!(entry["ready"], ready);
                let failing = if ready {
                    Vec::new()
                } else {
                    vec![failed.ok_or("missing failed dependency")?.name()]
                };
                assert_eq!(admission["failing"], serde_json::json!(failing));
                assert_eq!(entry["failing"], admission["failing"]);
            }
        }
        let mismatch = ReleaseGate {
            pending_package_semver: "pending-other-release".to_owned(),
            ..release
        };
        let reports = DependencyReports::new(|_| Ok("reachable".to_owned()));
        let readiness = HostedReadiness::assemble(&reports, &mismatch);
        assert!(!readiness.is_ready());
        let (_, document) = round_trip(json_response(503, readiness.document(&mismatch)))?;
        assert_eq!(document["release"]["state"], "degraded");
        assert!(Journey::ALL
            .into_iter()
            .all(|journey| JourneyReadiness::compute(journey, &reports).is_ready()));
        for retired in [
            "program-registry",
            "Programs",
            "settlement",
            "layerx-node-settlement",
        ] {
            assert!(!README.contains(retired), "retired README claim: {retired}");
        }
        assert_eq!(Journey::from_route("/v1/journeys/programs"), None);
        Ok(())
    }

    #[test]
    fn doc_testnet_readme_native_smoke_inputs() -> Result<(), String> {
        let mut configuration = platform_testnet();
        let pending = PendingRelease {
            package_semver: configuration.package_semver.clone(),
            wire_protocol_version: LXP_WIRE_PROTOCOL_VERSION,
        };
        assert_eq!(configuration.validate(&pending), Ok(()));
        assert_eq!(configuration.evm_chain_id, 125);
        configuration.evm_chain_id = u64::from(configuration.network_id);
        assert!(configuration.validate(&pending).is_err());
        for line in SMOKE.lines().filter(|line| line.starts_with(": \"${")) {
            let variable = line
                .trim_start_matches(": \"${")
                .split(':')
                .next()
                .ok_or("missing required input name")?;
            assert!(
                README.contains(variable),
                "undocumented required input: {variable}"
            );
        }
        let payment_call = SMOKE
            .split_once("/payment-journey.py\"")
            .ok_or("missing payment journey call")?
            .1
            .split_once("receipt_id=")
            .ok_or("missing receipt ID assignment")?
            .0;
        assert!(payment_call
            .contains("--evm-chain-id \"$(jq -er '.evm_chain_id' \"$work/parameters.json\")\""));
        assert!(!payment_call.contains("--network-id"));
        assert!(!payment_call.contains("--encoder"));
        assert!(!SMOKE.contains("LAYERX_TEST_SEND_ENCODER"));
        assert!(SMOKE.contains("and .evm_chain_id == 125"));
        assert!(PAYMENT.contains("parser.add_argument(\"--evm-chain-id\", type=int, choices=[EVM_CHAIN_ID], required=True)"));
        assert!(PAYMENT.contains("EVM_CHAIN_ID = 125"));
        assert!(PAYMENT.contains("NATIVE_DENOM = \"uhpx\""));
        assert!(PAYMENT.contains("NATIVE_SCALE = 10**12"));
        for value in [
            "0x0000000000000000000000000000000000001001",
            "px_requestFunds",
            "px_getBalances",
            "eth_chainId",
            "eth_sendRawTransaction",
            "already known",
        ] {
            assert!(PAYMENT.contains(value));
            assert!(
                README.contains(value),
                "missing native payment contract: {value}"
            );
        }
        for value in [
            "--evm-chain-id 125",
            "LAYERX_TEST_ASSET=uhpx",
            "10^12",
            "sendNative(string)",
        ] {
            assert!(
                README.contains(value),
                "missing native payment input: {value}"
            );
        }
        Ok(())
    }

    #[test]
    fn doc_testnet_readme_node_receipt_inspection() -> Result<(), String> {
        let inspection_call = SMOKE
            .split_once("admit_journey receipt-inspection")
            .ok_or("missing receipt inspection admission")?
            .1;
        assert!(
            inspection_call.contains("/receipt_inspection.py\" --node \"$LAYERX_TEST_NODE_URL\"")
        );
        assert!(inspection_call.contains("--receipt-keys \"$LAYERX_TEST_RECEIPT_KEYS_FILE\""));
        for value in [
            "payment.raw",
            "receipt.json",
            "result.json",
            "pax_sequencerReceipt",
            "verification.json",
        ] {
            assert!(INSPECTION.contains(value));
            assert!(
                README.contains(value),
                "missing receipt consumer contract: {value}"
            );
        }
        for value in [
            "LAYERX_TEST_NODE_URL",
            "LAYERX_TEST_RECEIPT_KEYS_FILE",
            "firstHeight",
            "lastHeight",
            "verification_level: sequencer-signed",
            "result_code: 0",
            "rung: committed",
            "HTTPS root node RPC",
            "independent Comet consensus proof",
        ] {
            assert!(README.contains(value), "missing receipt guidance: {value}");
        }
        assert!(INSPECTION.contains("url.path in (\"\", \"/\")"));
        assert!(INSPECTION.contains("verify_sequencer_receipt("));
        assert!(
            INSPECTION.contains("decode_receipt_keys(json.loads(args.receipt_keys.read_text()))")
        );
        assert!(INSPECTION.contains("key.first_height <= verified.height <= key.last_height"));
        assert!(INSPECTION.contains("error.check == \"bad_signature\""));
        assert!(SMOKE.contains(".verification_level == \"sequencer-signed\""));
        Ok(())
    }
}
