import { useCallback, useState } from "react";

export function useDepositQuantityInput(
  setDepositQuantity: (quantity: string) => void,
) {
  const [quantity, setInputQuantity] = useState("");

  const setQuantity = useCallback(
    (value: string) => {
      setInputQuantity(value);
      setDepositQuantity(value);
    },
    [setDepositQuantity],
  );

  return { quantity, setQuantity };
}
