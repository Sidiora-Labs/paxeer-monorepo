from pathlib import Path

from programme.kvx import load

SPEC = Path(__file__).resolve().parents[2] / "spec.kvx"


def test_inference_closure_requires_peer_loss_recovery() -> None:
    sections = load(SPEC)
    tasks = {
        name.removeprefix("task."): record
        for name, record in sections.items()
        if name.startswith("task.")
    }
    closure = "22.38.39"
    recovery = "22.38.20"
    assert recovery in tasks[closure]["requires"]
    assert {
        "22.38.37",
        "22.38.38",
        "22.38.33",
        "22.38.34",
        "22.38.35",
        "22.38.32",
        "22.38.13",
        "22.38.22",
        "22.38.29",
        "22.38.30",
    } <= set(tasks[closure]["requires"])
    assert tasks[recovery]["wave"] == 5
    assert tasks[closure]["wave"] == 9

    visited: set[str] = set()
    active: set[str] = set()
    stack = [(closure, False)]
    while stack:
        task_id, exiting = stack.pop()
        if exiting:
            active.remove(task_id)
            visited.add(task_id)
            continue
        assert task_id in tasks, f"Missing prerequisite task {task_id}"
        assert task_id not in active, f"Prerequisite cycle through {task_id}"
        if task_id in visited:
            continue
        active.add(task_id)
        prerequisites = tasks[task_id]["requires"]
        assert isinstance(prerequisites, list), task_id
        stack.append((task_id, True))
        stack.extend((prerequisite, False) for prerequisite in reversed(prerequisites))

    assert recovery in visited, f"Recovery {recovery} is unreachable from {closure}"
