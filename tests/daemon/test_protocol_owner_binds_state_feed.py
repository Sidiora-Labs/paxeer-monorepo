from recorded_paxd import (
    ROOT,
    RecordedPaxd,
    assert_in_order,
    programs_execution_case,
    source_function,
    verify_program_answer,
)


def test_protocol_owner_binds_the_state_feed():
    reset = source_function("modules/lxphost/host.go", r"^static int32_t lxphost_reset\(")
    assert_in_order(
        reset,
        "status = lx_account_registry_init(&node->accounts);",
        "lxp_state_store_bind_accounts(&node->store, &node->accounts)",
        "lxp_kernel_create(&node->kernel, &node->store, &node->journal, &node->parameters, epoch)",
        "programs_module_registration_v5() : programs_module_registration_v4_storage_retirement()",
    )
    binding = source_function(
        "modules/lxphost/host.go", r"^static int32_t lxphost_bind_assets\(",
    )
    assert_in_order(
        binding,
        "status = lx_asset_runtime_restore(&node->kernel, &node->assets, node->transfer_assets, &node->asset_runtime);",
        "if (status != LXP_OK) return status;",
        "node->programs_runtime.accounts = &node->accounts;",
        "node->programs_runtime.assets = node->transfer_assets;",
        "node->programs_runtime.asset_count = node->assets.count;",
        "node->programs_runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;",
        "node->programs_runtime.metering_schedule_context = &node->kernel;",
        "node->programs_runtime.resolve_occupancy_parameters = lxp_programs_fee_governance_resolve_runtime;",
        "node->programs_runtime.occupancy_parameter_context = &node->kernel;",
        "return lxp_kernel_bind_module_runtime(&node->kernel, LXP_MODULE_PROGRAMS, &node->programs_runtime);",
    )
    execute = source_function(
        "modules/lxphost/host.go", r"^func \(h \*Host\) HostExecute\(",
    )
    assert_in_order(
        execute,
        "node, err := h.borrowNode()",
        "defer h.returnNode(node)",
        "lxphost_reset(h.node, branch.sequence)",
        "branch.load(h, modules)",
        "h.loadBlobs(branch.store, modules)",
        "lxphost_bind_assets(h.node, h.cfg.NetworkID, bindingAuthority)",
        "lxphost_execute(h.node, h.cfg, branch, bindingAuthority, activity.ReceiptAuthorities...)",
        "branch.commit(h, did)",
        "commitBlobs(branch.store, loadedBlobs, stagedBlobs)",
    )
    kernel = (ROOT / "src/protocol/lxp_kernel.c").read_text()
    assert "lxp_programs_bind_state_feed(" not in kernel

    case = programs_execution_case()
    peer = RecordedPaxd.execution(case)
    with peer.serving() as client:
        tx_hash = client.send(case["rawTransaction"])
        answer = client.receipt(tx_hash)
        assert answer is not None
        verified = verify_program_answer(case, answer)
        assert verified.receipt.operation == 2
        assert len(verified.receipt.effects) == 2
