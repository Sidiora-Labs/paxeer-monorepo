from recorded_paxd import (
    ROOT,
    assert_in_order,
    observe_committed_program,
    recovery_recording,
    source_function,
)


def test_serve_startup_no_longer_binds_the_finality_authority():
    host = source_function("node/app.go", r"^func newHostWithProgramsABI\(")
    assert "return lxphost.NewHost(lxphost.Config{" in host
    assert "StoreKey: storeKey, NetworkID: networkID" in host
    assert "anchor" not in host.lower()
    app = (ROOT / "node/app.go").read_text()
    assert "lxp_finality_authority_bind" not in app
    assert "verify_finality_authority" not in app
    assert_in_order(
        app,
        "app.LayerXHost, err = NewHost(",
        "loadVersionHandler := func() error {",
        "app.LoadLatestVersion()",
        "app.refreshHostedKernelProgramsABI(ctx)",
    )
    upgrades = source_function("node/app.go", r"^func layerxStoreUpgrades\(")
    assert "case AnchorStoreDeleteUpgrade: return storetypes.StoreUpgrades{Deleted: []string{retiredAnchorStoreName}}, true" in upgrades
    merge = source_function("node/upgrades.go", r"^func mergeStoreUpgrades\(")
    assert "Deleted:" not in merge

    recording = recovery_recording()
    case = recording["case"]
    before = observe_committed_program(case, recording["phases"]["after_merge"], submit=True)
    # The second peer serves observations recorded from the reopened production
    # node. Restarting a replay listener alone is not evidence of node recovery.
    after = observe_committed_program(
        case, recording["phases"]["after_anchor_delete_reopen"], submit=False,
    )
    assert before == after
