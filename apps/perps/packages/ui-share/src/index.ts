import { registerShareDialog } from "./sharePnL/dialog";
import {
  SharePnLDialogWidget,
  SharePnLBottomSheetWidget,
} from "./sharePnL/sharePnL.widget";

const SharePnLDialogId = "sharePnLDialog";
const SharePnLBottomSheetId = "sharePnLBottomSheet";

registerShareDialog(SharePnLDialogId, SharePnLDialogWidget);
registerShareDialog(SharePnLBottomSheetId, SharePnLBottomSheetWidget);

export * from "./sharePnL";
export { SharePnLDialogId, SharePnLBottomSheetId };
export type {
  SharePnLConfig,
  SharePnLOptions,
  SharePnLParams,
} from "./types/types";
