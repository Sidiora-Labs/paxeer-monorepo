//! This module is used to validate the correctness of the gas metering algorithm.
//!
//! Since the gas metering algorithm is complex, this checks correctness using fixed WAT modules
//! covering structured control flow. The testing strategy is to ensure for all functions defined,
//! in all execution paths through the
//! function body that do not trap that the amount of gas charged by the proposed metering
//! instructions matches the expected translated fuel costs, including prepaid continuations.
//! This is done by constructing a control flow graph and exhaustively
//! searching through all paths, which may take exponential time in the size of the function body in
//! the worst case.

use super::{ConstantCostRules, MeteredBlock, Rules};
use parity_wasm::elements::{FuncBody, Instruction};
use std::collections::BTreeMap as Map;

/// An ID for a node in a ControlFlowGraph.
type NodeId = usize;

/// A node in a control flow graph is commonly known as a basic block. This is a sequence of
/// operations that are always executed sequentially.
#[derive(Debug, Default)]
struct ControlFlowNode {
    /// The index of the first instruction in the basic block. This is only used for debugging.
    first_instr_pos: Option<usize>,

    /// The expected translated gas cost attributed to this basic block.
    actual_cost: u64,

    /// The amount of gas charged by the injected metering instructions within this basic block.
    charged_cost: u64,

    /// Whether there are any other nodes in the graph that loop back to this one. Every cycle in
    /// the control flow graph contains at least one node with this flag set.
    is_loop_target: bool,

    /// Edges in the "forward" direction of the graph. The graph of nodes and their forward edges
    /// forms a directed acyclic graph (DAG).
    forward_edges: Vec<NodeId>,

    /// Edges in the "backwards" direction. These edges form cycles in the graph.
    loopback_edges: Vec<NodeId>,
}

/// A control flow graph where nodes are basic blocks and edges represent possible transitions
/// between them in execution flow. The graph has two types of edges, forward and loop-back edges.
/// The subgraph with only the forward edges forms a directed acyclic graph (DAG); including the
/// loop-back edges introduces cycles.
#[derive(Debug)]
pub struct ControlFlowGraph {
    nodes: Vec<ControlFlowNode>,
}

impl ControlFlowGraph {
    fn new() -> Self {
        ControlFlowGraph { nodes: Vec::new() }
    }

    fn get_node(&self, node_id: NodeId) -> &ControlFlowNode {
        self.nodes.get(node_id).unwrap()
    }

    fn get_node_mut(&mut self, node_id: NodeId) -> &mut ControlFlowNode {
        self.nodes.get_mut(node_id).unwrap()
    }

    fn add_node(&mut self) -> NodeId {
        self.nodes.push(ControlFlowNode::default());
        self.nodes.len() - 1
    }

    fn increment_actual_cost(&mut self, node_id: NodeId, cost: u32) {
        self.get_node_mut(node_id).actual_cost += u64::from(cost);
    }

    fn increment_charged_cost(&mut self, node_id: NodeId, cost: u64) {
        self.get_node_mut(node_id).charged_cost += cost;
    }

    fn set_first_instr_pos(&mut self, node_id: NodeId, first_instr_pos: usize) {
        self.get_node_mut(node_id).first_instr_pos = Some(first_instr_pos)
    }

    fn new_edge(&mut self, from_id: NodeId, target_frame: &ControlFrame) {
        if target_frame.is_loop {
            self.new_loopback_edge(from_id, target_frame.entry_node);
        } else {
            self.new_forward_edge(from_id, target_frame.exit_node);
        }
    }

    fn new_forward_edge(&mut self, from_id: NodeId, to_id: NodeId) {
        self.get_node_mut(from_id).forward_edges.push(to_id)
    }

    fn new_loopback_edge(&mut self, from_id: NodeId, to_id: NodeId) {
        self.get_node_mut(from_id).loopback_edges.push(to_id);
        self.get_node_mut(to_id).is_loop_target = true;
    }
}

/// A control frame is opened upon entry into a function and by the `block`, `if`, and `loop`
/// instructions and is closed by `end` instructions.
struct ControlFrame {
    is_loop: bool,
    if_origin: Option<NodeId>,
    entry_node: NodeId,
    exit_node: NodeId,
    active_node: NodeId,
}

impl ControlFrame {
    fn new(entry_node_id: NodeId, exit_node_id: NodeId, is_loop: bool) -> Self {
        ControlFrame {
            is_loop,
            if_origin: None,
            entry_node: entry_node_id,
            exit_node: exit_node_id,
            active_node: entry_node_id,
        }
    }
}

/// Construct a control flow graph from a function body, computed metered blocks and fixed
/// expected translated instruction costs, including prepaid continuations.
///
/// This assumes that the function body has been validated already, otherwise this may panic.
fn build_control_flow_graph(
    body: &FuncBody,
    rules: &impl Rules,
    blocks: &[MeteredBlock],
    expected_costs: &[u32],
) -> Result<ControlFlowGraph, ()> {
    assert_eq!(body.code().elements().len(), expected_costs.len());
    let mut graph = ControlFlowGraph::new();

    let entry_node_id = graph.add_node();
    let terminal_node_id = graph.add_node();

    graph.set_first_instr_pos(entry_node_id, 0);

    let mut stack = vec![ControlFrame::new(entry_node_id, terminal_node_id, false)];
    let mut metered_blocks_iter = blocks.iter().peekable();

    let locals_count = body
        .locals()
        .iter()
        .fold(0, |count, val_type| count + val_type.count());
    let locals_init_cost = rules.call_locals_cost(locals_count).ok_or(())?;

    for (cursor, instruction) in body.code().elements().iter().enumerate() {
        let active_node_id = stack
            .last()
            .expect("module is valid by pre-condition; control stack must not be empty; qed")
            .active_node;

        // Increment the charged cost if there are metering instructions to be inserted here.
        let apply_block =
            matches!(metered_blocks_iter.peek(), Some(block) if block.start_pos == cursor);
        if apply_block {
            let next_metered_block = metered_blocks_iter
                .next()
                .expect("peek returned an item; qed");
            graph.increment_charged_cost(active_node_id, next_metered_block.cost);
        }

        // Add locals initialization cost to the function block.
        if cursor == 0 {
            graph.increment_actual_cost(active_node_id, rules.block_entry_cost());
            graph.increment_actual_cost(active_node_id, locals_init_cost);
        }
        let instruction_cost = expected_costs[cursor];
        match instruction {
            Instruction::Block(_) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let exit_node_id = graph.add_node();
                stack.push(ControlFrame::new(active_node_id, exit_node_id, false));
            }
            Instruction::If(_) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let then_node_id = graph.add_node();
                let exit_node_id = graph.add_node();

                let mut frame = ControlFrame::new(then_node_id, exit_node_id, false);
                frame.if_origin = Some(active_node_id);
                stack.push(frame);
                graph.new_forward_edge(active_node_id, then_node_id);
                graph.increment_actual_cost(then_node_id, rules.block_entry_cost());
                graph.set_first_instr_pos(then_node_id, cursor + 1);
            }
            Instruction::Loop(_) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let loop_node_id = graph.add_node();
                let exit_node_id = graph.add_node();

                stack.push(ControlFrame::new(loop_node_id, exit_node_id, true));
                graph.new_forward_edge(active_node_id, loop_node_id);
                graph.increment_actual_cost(loop_node_id, rules.block_entry_cost());
                graph.set_first_instr_pos(loop_node_id, cursor + 1);
            }
            Instruction::Else => {
                let active_frame_idx = stack.len() - 1;
                let origin = stack[active_frame_idx].if_origin.take().ok_or(())?;
                graph.new_forward_edge(active_node_id, stack[active_frame_idx].exit_node);

                let else_node_id = graph.add_node();
                stack[active_frame_idx].active_node = else_node_id;
                graph.increment_actual_cost(else_node_id, instruction_cost);
                graph.increment_actual_cost(else_node_id, rules.block_entry_cost());

                graph.new_forward_edge(origin, else_node_id);
                graph.set_first_instr_pos(else_node_id, cursor + 1);
            }
            Instruction::End => {
                graph.increment_actual_cost(active_node_id, instruction_cost);
                let closing_frame = stack.pop()
					.expect("module is valid by pre-condition; ends correspond to control stack frames; qed");

                if let Some(origin) = closing_frame.if_origin {
                    graph.new_forward_edge(origin, closing_frame.exit_node);
                }
                graph.new_forward_edge(active_node_id, closing_frame.exit_node);
                graph.set_first_instr_pos(closing_frame.exit_node, cursor + 1);

                if let Some(active_frame) = stack.last_mut() {
                    active_frame.active_node = closing_frame.exit_node;
                }
            }
            Instruction::Br(label) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let active_frame_idx = stack.len() - 1;
                let target_frame_idx = active_frame_idx - (*label as usize);
                graph.new_edge(active_node_id, &stack[target_frame_idx]);

                // Next instruction is unreachable, but carry on anyway.
                let new_node_id = graph.add_node();
                stack[active_frame_idx].active_node = new_node_id;
                graph.set_first_instr_pos(new_node_id, cursor + 1);
            }
            Instruction::BrIf(label) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let active_frame_idx = stack.len() - 1;
                let target_frame_idx = active_frame_idx - (*label as usize);
                graph.new_edge(active_node_id, &stack[target_frame_idx]);

                let new_node_id = graph.add_node();
                stack[active_frame_idx].active_node = new_node_id;
                graph.new_forward_edge(active_node_id, new_node_id);
                graph.set_first_instr_pos(new_node_id, cursor + 1);
            }
            Instruction::BrTable(br_table_data) => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                let active_frame_idx = stack.len() - 1;
                for &label in [br_table_data.default]
                    .iter()
                    .chain(br_table_data.table.iter())
                {
                    let target_frame_idx = active_frame_idx - (label as usize);
                    graph.new_edge(active_node_id, &stack[target_frame_idx]);
                }

                let new_node_id = graph.add_node();
                stack[active_frame_idx].active_node = new_node_id;
                graph.set_first_instr_pos(new_node_id, cursor + 1);
            }
            Instruction::Return => {
                graph.increment_actual_cost(active_node_id, instruction_cost);

                graph.new_forward_edge(active_node_id, terminal_node_id);

                let active_frame_idx = stack.len() - 1;
                let new_node_id = graph.add_node();
                stack[active_frame_idx].active_node = new_node_id;
                graph.set_first_instr_pos(new_node_id, cursor + 1);
            }
            Instruction::Unreachable => {
                graph.increment_actual_cost(active_node_id, instruction_cost);
                let active_frame_idx = stack.len() - 1;
                let new_node_id = graph.add_node();
                stack[active_frame_idx].active_node = new_node_id;
                graph.set_first_instr_pos(new_node_id, cursor + 1);
            }
            _ => graph.increment_actual_cost(active_node_id, instruction_cost),
        }
    }

    assert!(stack.is_empty());

    Ok(graph)
}

/// Exhaustively search through all paths in the control flow graph, starting from the first node
/// and ensure that 1) all paths with only forward edges ending with the terminal node have an
/// equal total actual gas cost and total charged gas cost, and 2) all cycles beginning with a loop
/// entry point and ending with a node with a loop-back edge to the entry point have equal actual
/// and charged gas costs. If this returns true, then the metered blocks used to construct the
/// control flow graph are correct with respect to the function body.
///
/// In the worst case, this runs in time exponential in the size of the graph.
fn validate_graph_gas_costs(graph: &ControlFlowGraph) -> bool {
    fn visit(
        graph: &ControlFlowGraph,
        node_id: NodeId,
        mut total_actual: u64,
        mut total_charged: u64,
        loop_costs: &mut Map<NodeId, (u64, u64)>,
    ) -> bool {
        let node = graph.get_node(node_id);

        if node.is_loop_target {
            loop_costs.insert(node_id, (total_actual, total_charged));
        }

        total_actual += node.actual_cost;
        total_charged += node.charged_cost;

        if node.forward_edges.is_empty() && total_actual != total_charged {
            return false;
        }

        for loop_node_id in node.loopback_edges.iter() {
            let (loop_actual, loop_charged) = loop_costs
                .get(loop_node_id)
                .expect("cannot arrive at loopback edge without visiting loop entry node");
            if total_actual - loop_actual != total_charged - loop_charged {
                return false;
            }
        }

        for next_node_id in node.forward_edges.iter() {
            if !visit(
                graph,
                *next_node_id,
                total_actual,
                total_charged,
                loop_costs,
            ) {
                return false;
            }
        }

        if node.is_loop_target {
            loop_costs.remove(&node_id);
        }

        true
    }

    // Recursively explore all paths through the execution graph starting from the entry node.
    visit(graph, 0, 0, 0, &mut Map::new())
}

/// Validate that the metered blocks are correct with respect to the function body by exhaustively
/// searching all paths through the control flow graph.
///
/// This assumes that the function body has been validated already, otherwise this may panic.
fn validate_metering_injections(
    body: &FuncBody,
    rules: &impl Rules,
    blocks: &[MeteredBlock],
    expected_costs: &[u32],
) -> Result<bool, ()> {
    let graph = build_control_flow_graph(body, rules, blocks, expected_costs)?;
    Ok(validate_graph_gas_costs(&graph))
}

mod tests {
    use super::{super::determine_metered_blocks, *};

    use parity_wasm::elements;

    #[test]
    fn test_build_control_flow_graph() {
        // dfffe1310 prepays control-frame continuations across conditional branches.
        // Freeze that translated cost model independently of determine_metered_blocks.
        type ExpectedFunction<'a> = (&'a [u32], &'a [(usize, u64)]);
        let modules: &[(&str, &[ExpectedFunction<'_>])] = &[
            (
                r#"(module (func (result i32)
                (block (result i32)
                    (block (result i32) i32.const 7)
                    i32.const 3 i32.add)))"#,
                &[(&[1, 1, 1, 1, 1, 1, 1, 1], &[(0, 8)])],
            ),
            (
                r#"(module (func (param i32)
                (loop $again
                    local.get 0 i32.const 1 i32.sub local.tee 0
                    br_if $again)))"#,
                &[(&[2, 1, 1, 1, 1, 2, 0, 0], &[(0, 2), (1, 6)])],
            ),
            (
                r#"(module (func (result i32)
                (block $exit (result i32)
                    i32.const 7 br $exit
                    i32.const 9)))"#,
                &[(&[1, 1, 1, 0, 0, 1], &[(0, 4)])],
            ),
            (
                r#"(module (func (param i32) (result i32)
                (block $exit
                    local.get 0 br_if $exit
                    i32.const 3 local.set 0)
                local.get 0))"#,
                &[(&[1, 1, 4, 0, 0, 0, 1, 1], &[(0, 8)])],
            ),
            (
                r#"(module (func (param i32) (result i32)
                (block $outer
                    (block $inner
                        local.get 0 br_table $inner $outer $outer)
                    i32.const 7 local.set 0)
                local.get 0))"#,
                &[(&[1, 1, 1, 1, 0, 1, 1, 1, 1, 1], &[(0, 6), (5, 3)])],
            ),
            (
                r#"(module (func (param i32) (result i32)
                local.get 0
                (if (result i32)
                    (then i32.const 7)
                    (else i32.const 9))))"#,
                &[(&[2, 1, 1, 1, 1, 1, 0], &[(0, 3), (2, 1), (4, 3)])],
            ),
            (
                r#"(module
                (func $callee (param i32) (result i32)
                    local.get 0 i32.const 1 i32.add)
                (func (param i32) (result i32)
                    local.get 0 call $callee))"#,
                &[(&[1, 1, 1, 1], &[(0, 4)]), (&[1, 1, 1], &[(0, 3)])],
            ),
            (
                r#"(module (func (param i32) (result i32)
                local.get 0
                (if (then i32.const 7 return))
                i32.const 9 return
                i32.const 11))"#,
                &[(&[1, 1, 1, 1, 0, 1, 1, 0, 0], &[(0, 2), (2, 2), (5, 2)])],
            ),
        ];
        for &(text, expected_functions) in modules {
            let module_bytes = wat::parse_str(text).expect("failed to parse fixed WAT module");
            let module: elements::Module = elements::deserialize_buffer(&module_bytes)
                .expect("failed to parse Wasm blob generated from fixed WAT");

            let functions = module.code_section().unwrap().bodies();
            assert_eq!(functions.len(), expected_functions.len());
            for (func_body, &(expected_costs, expected_blocks)) in
                functions.iter().zip(expected_functions)
            {
                let rules = ConstantCostRules::default();
                let locals_count = func_body
                    .locals()
                    .iter()
                    .fold(0, |count, val_type| count + val_type.count());

                let metered_blocks =
                    determine_metered_blocks(func_body.code(), &rules, locals_count, 0).unwrap();
                assert_eq!(
                    metered_blocks
                        .iter()
                        .map(|block| (block.start_pos, block.cost))
                        .collect::<Vec<_>>(),
                    expected_blocks,
                    "unexpected metering schedule for WAT module {text}",
                );
                let success = validate_metering_injections(
                    func_body,
                    &rules,
                    &metered_blocks,
                    expected_costs,
                )
                .unwrap();
                assert!(
                    success,
                    "metering mismatch for WAT module {text}: blocks {metered_blocks:?}; graph {:#?}",
                    build_control_flow_graph(func_body, &rules, &metered_blocks, expected_costs).unwrap(),
                );
                for changed_block in 0..metered_blocks.len() {
                    let mut incorrect_blocks = metered_blocks
                        .iter()
                        .map(|block| MeteredBlock {
                            start_pos: block.start_pos,
                            cost: block.cost,
                        })
                        .collect::<Vec<_>>();
                    incorrect_blocks[changed_block].cost += 1;
                    assert!(
                        !validate_metering_injections(
                            func_body,
                            &rules,
                            &incorrect_blocks,
                            expected_costs
                        )
                        .unwrap(),
                        "accepted incorrect cost at block {changed_block} for WAT module {text}",
                    );
                }
            }
        }
    }
}
