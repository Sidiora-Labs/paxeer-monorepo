import { renderStatus } from "./peer/src/page.mjs";
import frames from "./peer/room.json" with { type: "json" };

const state = new URL(location.href).searchParams.get("state");

async function waitFor(predicate) {
  if (predicate()) return;
  await new Promise((resolve, reject) => {
    const observer = new MutationObserver(() => {
      if (!predicate()) return;
      observer.disconnect();
      clearTimeout(timeout);
      resolve();
    });
    const timeout = setTimeout(() => {
      observer.disconnect();
      reject(new Error("The page did not reach its requested state"));
    }, 8000);
    observer.observe(document, {
      subtree: true,
      childList: true,
      characterData: true,
    });
  });
}

if (state === "room-status") {
  const offer = frames.find((frame) => frame.type === "offer");
  const peerKey = offer.offer.layer_plan[0].peer_key
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
  function showRoom() {
    for (const frame of frames.filter((frame) =>
      ["offer", "deal"].includes(frame.type),
    )) {
      renderStatus(document, frame, peerKey);
    }
  }
  showRoom();
  for (const detail of document.querySelectorAll("#status-heading ~ details"))
    detail.open = true;
  await waitFor(() =>
    [
      "Device setup failed",
      "Measure this device before joining a room",
    ].includes(document.querySelector("#connection").textContent),
  );
  showRoom();
}
if (state === "print-error" || state === "print-empty") {
  const output = document.querySelector("#output");
  await waitFor(() => output.textContent.includes("(module"));
  const input = document.querySelector("#fileInput");
  document.querySelector("#upload").click();
  const transfer = new DataTransfer();
  const bytes =
    state === "print-empty"
      ? new Uint8Array([0, 97, 115, 109, 1, 0, 0, 0])
      : new Uint8Array([0]);
  transfer.items.add(
    new File([bytes], "module.wasm", { type: "application/wasm" }),
  );
  input.files = transfer.files;
  input.dispatchEvent(new Event("change"));
  await waitFor(() =>
    state === "print-empty"
      ? output.textContent.includes("(module)")
      : output.textContent.includes("unexpected end"),
  );
}
if (state === "print-drop") {
  await waitFor(() =>
    document.querySelector("#output").textContent.includes("(module"),
  );
  dispatchEvent(
    new DragEvent("dragover", {
      bubbles: true,
      cancelable: true,
      dataTransfer: new DataTransfer(),
    }),
  );
}
