import { FC } from "react";
import {
  useDepositFormScript,
  DepositFormScriptOptions,
} from "./depositForm.script";
import { DepositProvider } from "../provider/DepositProvider";
import { DepositForm } from "./depositForm.ui";

export type DepositFormWidgetProps = DepositFormScriptOptions;

export const DepositFormWidget: FC<DepositFormWidgetProps> = (props) => {
  const state = useDepositFormScript(props);
  return (
    <DepositProvider collateralUnit={state.collateralUnit}>
      <DepositForm {...state} />
    </DepositProvider>
  );
};
