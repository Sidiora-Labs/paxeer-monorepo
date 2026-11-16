const scenario = new URL(location.href).searchParams.get("fixture");
if (scenario === "sign-in-storage") {
  const opening = indexedDB.open("layerx-developer", 2);
  opening.onupgradeneeded = () => opening.result.createObjectStore("keys");
  opening.onsuccess = () => opening.result.close();
}
if (scenario === "sign-in-key-loading") {
  const opening = indexedDB.open("layerx-developer", 1);
  opening.onupgradeneeded = () => {
    const store = opening.result.createObjectStore("keys");
    function keepUpgradeOpen() {
      store.get("developer").onsuccess = keepUpgradeOpen;
    }
    keepUpgradeOpen();
  };
}
if (scenario === "keys-secret") {
  const opening = indexedDB.open("layerx-developer", 1);
  opening.onupgradeneeded = () => opening.result.createObjectStore("keys");
  opening.onsuccess = async () => {
    try {
      const pair = await crypto.subtle.generateKey("Ed25519", false, [
        "sign",
        "verify",
      ]);
      const transaction = opening.result.transaction("keys", "readwrite");
      transaction.objectStore("keys").put(pair, "developer");
      transaction.oncomplete = () => opening.result.close();
      transaction.onerror = () => opening.result.close();
    } catch {
      opening.result.close();
    }
  };
}
